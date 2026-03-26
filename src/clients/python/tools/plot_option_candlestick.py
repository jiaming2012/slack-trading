#!/usr/bin/env python3
"""
Plot candlestick chart with option trade overlays.

Fetch mode (primary):
    python plot_option_candlestick.py --playground-id <UUID> --symbol AAPL \\
        [--twirp-url http://127.0.0.1:5051] [--timeframe 3600]

Legacy JSON mode:
    python plot_option_candlestick.py --json '<json-blob>'
"""

import re
import sys
import json
import argparse
import concurrent.futures
import numpy as np
import pandas as pd
import plotly.graph_objects as go
from plotly.subplots import make_subplots
from dateutil.parser import isoparse, parse as dateutil_parse
from zoneinfo import ZoneInfo
from twirp.context import Context
from typing import Dict, Any, List, Optional, Tuple

from rpc.playground_twirp import PlaygroundServiceClient
from rpc.playground_pb2 import (
    GetPlaygroundsRequest,
    GetAccountRequest,
    GetCandlesRequest,
)


# ------------------------------------------------------------------ #
# Fetch helpers
# ------------------------------------------------------------------ #

def _to_rfc3339(date_str: str) -> str:
    """Convert any recognisable date string to a UTC RFC3339 string."""
    if not date_str:
        return ""
    try:
        dt = isoparse(date_str)
    except Exception:
        try:
            dt = dateutil_parse(date_str.rsplit(' ', 1)[0])
        except Exception:
            return date_str
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=ZoneInfo('UTC'))
    return dt.astimezone(ZoneInfo('UTC')).strftime("%Y-%m-%dT%H:%M:%S") + "Z"


def _parse_strike_from_symbol(option_symbol: str) -> Optional[float]:
    """Extract the strike price from a symbol such as O:AAPL241220C00200000."""
    sym = option_symbol[2:] if option_symbol.startswith('O:') else option_symbol
    m = re.match(r'^[A-Z]+\d{6}[CP](\d{8})$', sym.upper())
    return float(m.group(1)) / 1000.0 if m else None


_MONTHS = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

def _short_option_label(symbol: str) -> str:
    """Convert 'O:AAPL250103P00245000' → 'Jan 03 Put $245' for dropdown labels."""
    sym = symbol[2:] if symbol.startswith('O:') else symbol
    m = re.match(r'^[A-Z]+(\d{2})(\d{2})(\d{2})([CP])(\d{8})$', sym.upper())
    if not m:
        return symbol
    yy, mm, dd, opt_type, strike_raw = m.groups()
    strike = float(strike_raw) / 1000.0
    month = _MONTHS[int(mm) - 1]
    kind = 'Put' if opt_type == 'P' else 'Call'
    return f"{month} {dd} {kind} ${strike:.0f}"


def _has_field(msg, field: str) -> bool:
    """Safe wrapper around protobuf HasField (works for optional scalars)."""
    try:
        return msg.HasField(field)
    except (ValueError, AttributeError):
        return False


def _order_fill_date(order) -> str:
    return order.trades[0].create_date if order.trades else order.create_date


def _order_fill_price(order) -> float:
    if order.trades:
        return sum(t.price for t in order.trades) / len(order.trades)
    return order.price


def _order_type(order) -> str:
    side = order.side.lower()
    return 'Buy' if side in ('buy', 'buy_to_close', 'buy_to_cover') else 'Sell'


def _build_candle_data(bars) -> Dict[str, Any]:
    return {
        'Date':  [b.datetime for b in bars],
        'Open':  [b.open     for b in bars],
        'High':  [b.high     for b in bars],
        'Low':   [b.low      for b in bars],
        'Close': [b.close    for b in bars],
    }


def _build_option_data(bars) -> Dict[str, Any]:
    return {
        'Date':  [b.datetime for b in bars],
        'Open':  [b.open     for b in bars],
        'Close': [b.close    for b in bars],
    }


def _build_equity_order_data(
    equity_orders: list,
    option_orders: list,
) -> Dict[str, Any]:
    """
    Build order_data for equity trades.

    Strike reference lines (A / B) are derived from the traded option strikes.
    PairId links a buy to its matching sell via the close_order_id field.
    """
    strikes = sorted(filter(None, (
        _parse_strike_from_symbol(o.symbol) for o in option_orders
    )))
    strike_a = strikes[0] if len(strikes) > 0 else 0.0
    strike_b = strikes[1] if len(strikes) > 1 else strike_a

    base = {'Date': [], 'Price': [], 'Type': [],
            'StrikePriceA': strike_a, 'StrikePriceB': strike_b}

    if not equity_orders:
        return base

    dates, prices, types, pair_ids, pnls = [], [], [], [], []
    for order in sorted(equity_orders, key=_order_fill_date):
        dates.append(_order_fill_date(order))
        prices.append(_order_fill_price(order))
        types.append(_order_type(order))

        side = order.side.lower()
        if side in ('buy', 'sell_short'):
            pair_ids.append(str(order.id))
            pnls.append(None)
        elif side in ('sell', 'buy_to_cover') and _has_field(order, 'close_order_id'):
            pair_ids.append(str(order.close_order_id))
            pnls.append(float(order.pl) if _has_field(order, 'pl') else None)
        else:
            pair_ids.append(str(order.id))
            pnls.append(None)

    return {**base,
            'Date': dates, 'Price': prices, 'Type': types,
            'PairId': pair_ids, 'PnL': pnls}


def _build_option_order_data(option_orders: list) -> Dict[str, Any]:
    """
    Build option_order_data with PairId and computed PnL.

    PnL per contract = (open_premium - close_premium) x quantity x 100.
    Uses order.pl when available; otherwise computed from fill prices.
    """
    if not option_orders:
        return {'Date': [], 'Price': [], 'Type': [], 'Symbol': []}

    open_prices: Dict[int, float] = {
        o.id: _order_fill_price(o)
        for o in option_orders
        if o.side.lower() == 'sell_to_open'
    }

    dates, prices, types, symbols, pair_ids, pnls = [], [], [], [], [], []
    for order in sorted(option_orders, key=_order_fill_date):
        fill = _order_fill_price(order)
        dates.append(_order_fill_date(order))
        prices.append(fill)
        types.append(_order_type(order))
        symbols.append(order.symbol)
        side = order.side.lower()

        if side == 'sell_to_open':
            pair_ids.append(str(order.id))
            pnls.append(None)
        elif side == 'buy_to_close' and _has_field(order, 'close_order_id'):
            pair_ids.append(str(order.close_order_id))
            open_p = open_prices.get(order.close_order_id)
            if open_p is not None:
                pnls.append(round((open_p - fill) * order.quantity * 100, 2))
            elif _has_field(order, 'pl'):
                pnls.append(float(order.pl))
            else:
                pnls.append(None)
        else:
            pair_ids.append(str(order.id))
            pnls.append(None)

    return {'Date': dates, 'Price': prices, 'Type': types, 'Symbol': symbols,
            'PairId': pair_ids, 'PnL': pnls}


def fetch_playground_data(
    playground_id: str,
    symbol: str,
    timeframe: int,
    twirp_url: str,
    option_symbol: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Fetch all data needed for plotting from a running playground server and
    return a dict in the same shape expected by plot_candlestick().
    """
    client = PlaygroundServiceClient(twirp_url, timeout=120)
    ctx = Context()

    # 1. Resolve playground metadata
    pg_resp = client.GetPlaygrounds(ctx=ctx, request=GetPlaygroundsRequest())
    session = next(
        (p for p in pg_resp.playgrounds if p.playground_id == playground_id),
        None,
    )
    if session is None:
        raise ValueError(f"Playground {playground_id!r} not found on {twirp_url}")

    # Prefer clock fields (always populated); fall back to meta fields
    start_rfc = _to_rfc3339(session.clock.start or session.meta.start_date)
    end_rfc   = _to_rfc3339(
        session.clock.stop
        or session.meta.end_date
        or session.clock.current_time
    )

    # 2. Fetch all orders
    acct = client.GetAccount(ctx=ctx, request=GetAccountRequest(
        playground_id=playground_id,
        fetch_orders=True,
    ))
    all_orders    = list(acct.orders)
    equity_orders = [o for o in all_orders
                     if getattr(o, 'class') == 'equity' and o.symbol == symbol]
    option_orders = [o for o in all_orders
                     if getattr(o, 'class') == 'option']

    # 3. Fetch underlying candles
    candle_resp = client.GetCandlesFromRepo(ctx=ctx, request=GetCandlesRequest(
        playground_id=playground_id,
        symbol=symbol,
        period_in_seconds=timeframe,
        fromRTF3339=start_rfc,
        toRTF3339=end_rfc,
    ))

    # 4. Fetch option candles for ALL traded symbols in parallel (best-effort)
    opened_option_symbols = sorted(
        {o.symbol for o in option_orders if o.side.lower() == 'sell_to_open'},
        key=lambda s: min(_order_fill_date(o) for o in option_orders if o.symbol == s),
    )

    if opened_option_symbols:
        print(f"Fetching candle data for {len(opened_option_symbols)} option symbols...",
              file=sys.stderr)

    if option_symbol and option_symbol not in opened_option_symbols:
        print(f"WARNING: --option-symbol {option_symbol!r} not found; ignoring.", file=sys.stderr)
        option_symbol = None

    def _fetch_one(sym: str):
        for rpc_fn in (client.GetCandlesFromRepo, client.GetCandlesFromDataSource):
            try:
                resp = rpc_fn(ctx=ctx, request=GetCandlesRequest(
                    playground_id=playground_id,
                    symbol=sym,
                    period_in_seconds=timeframe,
                    fromRTF3339=start_rfc,
                    toRTF3339=end_rfc,
                ))
                if resp.bars:
                    return sym, list(resp.bars)
            except Exception:
                pass
        return sym, []

    # Candle data for option symbols — best-effort (dates may be corrupt)
    option_data_by_symbol: Dict[str, Dict] = {}
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as pool:
        for sym, bars in pool.map(_fetch_one, opened_option_symbols):
            if bars:
                option_data_by_symbol[sym] = _build_option_data(bars)

    # Per-symbol order data for the dropdown
    option_order_data_by_symbol: Dict[str, Dict] = {
        sym: _build_option_order_data([o for o in option_orders if o.symbol == sym])
        for sym in opened_option_symbols
    }

    default_sym = option_symbol or (opened_option_symbols[0] if opened_option_symbols else None)
    opt_subtitle = f"Options ({len(opened_option_symbols)} symbols)"

    return {
        'chart_data': {
            'title':                   f"{symbol} — playground {playground_id[:8]}...",
            'subplot_1_title':         f"{symbol} ({timeframe}s candles)",
            'subplot_2_title':         opt_subtitle,
            'timeframe':               timeframe,
            'default_option_symbol':   default_sym,
        },
        'candle_data':                   _build_candle_data(candle_resp.bars),
        'order_data':                    _build_equity_order_data(equity_orders, option_orders),
        'option_data':                   _build_option_data([]),   # legacy compat
        'option_order_data':             _build_option_order_data(option_orders),
        'option_data_by_symbol':         option_data_by_symbol,
        'option_order_data_by_symbol':   option_order_data_by_symbol,
    }


# ------------------------------------------------------------------ #
# Plotting helpers
# ------------------------------------------------------------------ #

def build_trade_pairs(df: pd.DataFrame) -> List[Dict]:
    """
    Group orders into open/close pairs via the PairId column.

    Returns a list of dicts with keys:
        pair_id, open_date, open_price, close_date, close_price, pnl
    Pairs with only one entry (still-open positions) are skipped.
    """
    if 'PairId' not in df.columns:
        return []

    pairs = []
    for pair_id, group in df.groupby('PairId'):
        group = group.sort_values('Date')
        if len(group) < 2:
            continue
        open_row  = group.iloc[0]
        close_row = group.iloc[-1]
        pnl: Optional[float] = None
        if 'PnL' in group.columns:
            vals = group['PnL'].dropna()
            if not vals.empty:
                pnl = float(vals.iloc[-1])
        pairs.append({
            'pair_id':     pair_id,
            'open_date':   open_row['Date'],
            'open_price':  float(open_row['Price']),
            'close_date':  close_row['Date'],
            'close_price': float(close_row['Price']),
            'pnl':         pnl,
        })
    return pairs


def add_holding_period_shading(fig: go.Figure, pairs: List[Dict]) -> None:
    """
    Add a semi-transparent vertical band for each trade's holding period.
    Green-tinted for profitable trades, red-tinted for losing trades.
    Spans all subplot rows.
    """
    for pair in pairs:
        if pair['pnl'] is not None:
            fill = 'rgba(0, 180, 0, 0.07)' if pair['pnl'] > 0 else 'rgba(200, 0, 0, 0.07)'
        else:
            fill = 'rgba(120, 120, 120, 0.07)'
        fig.add_vrect(
            x0=pair['open_date'], x1=pair['close_date'],
            fillcolor=fill, opacity=1.0, layer='below', line_width=0,
        )


def add_trade_pair_connectors(fig: go.Figure, pairs: List[Dict], row: int) -> None:
    """
    Draw one colored line per trade pair connecting open -> close on the given row.

    Green  = profitable (pnl > 0)
    Red    = losing     (pnl <= 0)
    Gray   = pnl unknown

    One legend entry per outcome category; P&L label at the midpoint.
    P&L labels are added as a scatter text trace (not annotations) so they are
    subject to the dropdown visibility array and hidden/shown with their group.
    """
    legend_shown = {'profit': False, 'loss': False, 'unknown': False}
    label_x: List = []
    label_y: List = []
    label_text: List = []
    label_colors: List = []

    for pair in pairs:
        pnl = pair['pnl']
        if pnl is None:
            color, category, legend_name = 'gray', 'unknown', 'Trade (P&L unknown)'
        elif pnl > 0:
            color, category, legend_name = 'green', 'profit', 'Profitable Trade'
        else:
            color, category, legend_name = 'red', 'loss', 'Losing Trade'

        show_legend = not legend_shown[category]
        legend_shown[category] = True

        fig.add_trace(go.Scatter(
            x=[pair['open_date'], pair['close_date']],
            y=[pair['open_price'], pair['close_price']],
            mode='lines',
            line=dict(color=color, width=1.5),
            name=legend_name,
            legendgroup=legend_name,
            showlegend=show_legend,
        ), row=row, col=1)

        if pnl is not None:
            mid_date  = pair['open_date'] + (pair['close_date'] - pair['open_date']) / 2
            mid_price = (pair['open_price'] + pair['close_price']) / 2
            label     = f"+${pnl:.2f}" if pnl > 0 else f"-${abs(pnl):.2f}"
            label_x.append(mid_date)
            label_y.append(mid_price)
            label_text.append(label)
            label_colors.append(color)

    if label_x:
        fig.add_trace(go.Scatter(
            x=label_x, y=label_y,
            mode='text',
            text=label_text,
            textfont=dict(size=9, family='monospace', color=label_colors),
            textposition='top center',
            showlegend=False,
            hoverinfo='skip',
            name='P&L Labels',
        ), row=row, col=1)


# ------------------------------------------------------------------ #
# Core plot function
# ------------------------------------------------------------------ #

def _build_underlying_markers(
    df_opt_orders: pd.DataFrame,
    df_candles: pd.DataFrame,
) -> pd.DataFrame:
    """
    For each option order fill, look up the underlying close price at that timestamp.
    Returns columns: Date, Price (underlying), Type, Symbol, OptionPrice, PnL, PairId.
    PairId is carried through from df_opt_orders so build_trade_pairs() can pair them.
    """
    cols = ['Date', 'Price', 'Type', 'Symbol', 'OptionPrice', 'PnL', 'PairId']
    if df_opt_orders.empty or df_candles.empty:
        return pd.DataFrame(columns=cols)

    candles_sorted = df_candles.sort_values('Date').reset_index(drop=True)
    has_sym    = 'Symbol' in df_opt_orders.columns
    has_pnl    = 'PnL'    in df_opt_orders.columns
    has_pairid = 'PairId' in df_opt_orders.columns

    rows = []
    for _, row in df_opt_orders.iterrows():
        target = row['Date']
        if pd.isna(target):
            continue
        idx = int(candles_sorted['Date'].searchsorted(target, side='right')) - 1
        if idx < 0:
            idx = 0
        underlying_price = float(candles_sorted.iloc[idx]['Close'])
        rows.append({
            'Date':        target,
            'Price':       underlying_price,
            'Type':        row['Type'],
            'Symbol':      row['Symbol'] if has_sym else '',
            'OptionPrice': row['Price'],   # option premium at fill
            'PnL':         row['PnL']    if has_pnl    else None,
            'PairId':      row['PairId'] if has_pairid else '',
        })

    return pd.DataFrame(rows) if rows else pd.DataFrame(columns=cols)


def _opt_customdata(df_sub: pd.DataFrame) -> np.ndarray:
    """Build customdata [[symbol, short_label, pnl_str], ...] for option order markers."""
    rows = []
    has_sym = 'Symbol' in df_sub.columns
    has_pnl = 'PnL' in df_sub.columns
    for _, row in df_sub.iterrows():
        sym = row['Symbol'] if has_sym else ''
        label = _short_option_label(str(sym)) if sym else ''
        pnl = row['PnL'] if has_pnl else None
        pnl_str = f"${float(pnl):+.2f}" if pnl is not None and not pd.isna(pnl) else 'open'
        rows.append([sym, label, pnl_str])
    return np.array(rows, dtype=object) if rows else np.empty((0, 3), dtype=object)


_OPT_HOVER = (
    '<b>%{customdata[1]}</b><br>'
    'Symbol: %{customdata[0]}<br>'
    'Price: %{y:.4f}<br>'
    'P&L: %{customdata[2]}<br>'
    'Date: %{x}<extra></extra>'
)

def _und_customdata(df_sub: pd.DataFrame) -> np.ndarray:
    """Build customdata [[symbol, label, opt_price_str, pnl_str], ...] for underlying markers."""
    rows = []
    has_sym = 'Symbol' in df_sub.columns
    has_opt = 'OptionPrice' in df_sub.columns
    has_pnl = 'PnL' in df_sub.columns
    for _, row in df_sub.iterrows():
        sym = row['Symbol'] if has_sym else ''
        label = _short_option_label(str(sym)) if sym else ''
        opt_price = row['OptionPrice'] if has_opt else 0.0
        pnl = row['PnL'] if has_pnl else None
        pnl_str = f"${float(pnl):+.2f}" if pnl is not None and not pd.isna(pnl) else 'open'
        rows.append([sym, label, f"{opt_price:.4f}", pnl_str])
    return np.array(rows, dtype=object) if rows else np.empty((0, 4), dtype=object)


_UND_OPT_HOVER = (
    '<b>%{customdata[1]}</b><br>'
    'Symbol: %{customdata[0]}<br>'
    'Underlying: $%{y:.2f}<br>'
    'Option Premium: $%{customdata[2]}<br>'
    'P&L: %{customdata[3]}<br>'
    'Date: %{x}<extra></extra>'
)

def plot_candlestick(
    chart_title: str,
    subplot1_title: str,
    subplot2_title: str,
    candle_data: Dict[str, Any],
    order_data: Dict[str, Any],
    option_data: Dict[str, Any],
    option_order_data: Dict[str, Any],
    timeframe: int,
    option_data_by_symbol: Optional[Dict[str, Dict]] = None,
    default_option_symbol: Optional[str] = None,
    option_order_data_by_symbol: Optional[Dict[str, Dict]] = None,
) -> None:
    df = candle_to_np(candle_data, timeframe)
    df['High'] = df[['Open', 'High', 'Low', 'Close']].max(axis=1)
    df['Low']  = df[['Open', 'High', 'Low', 'Close']].min(axis=1)

    df_orders        = order_to_np(order_data)
    df_option_orders = order_to_np(option_order_data)

    # Normalise option_data_by_symbol: fall back to legacy single option_data
    if not option_data_by_symbol:
        df_single = option_to_np(option_data)
        option_data_by_symbol = {'Options': option_data} if not df_single.empty else {}
        default_option_symbol = 'Options' if option_data_by_symbol else None

    strike_price_a = order_data.get('StrikePriceA', 0.0)
    strike_price_b = order_data.get('StrikePriceB', 0.0)

    fig = make_subplots(
        rows=2, cols=1, shared_xaxes=True,
        vertical_spacing=0.2,
        subplot_titles=(subplot1_title, subplot2_title),
    )

    # ------------------------------------------------------------------ #
    # Row 1: underlying candlestick + equity orders
    # ------------------------------------------------------------------ #
    fig.add_trace(go.Candlestick(
        x=df['Date'],
        open=df['Open'], high=df['High'], low=df['Low'], close=df['Close'],
        increasing_line_color='green', decreasing_line_color='red',
        name='Underlying Candle',
    ), row=1, col=1)

    buy_orders  = df_orders[df_orders['Type'] == 'Buy']
    sell_orders = df_orders[df_orders['Type'] == 'Sell']

    fig.add_trace(go.Scatter(
        x=buy_orders['Date'], y=buy_orders['Price'], mode='markers',
        marker=dict(symbol='triangle-up', size=10, color='blue'),
        name='Buy Orders',
    ), row=1, col=1)

    fig.add_trace(go.Scatter(
        x=sell_orders['Date'], y=sell_orders['Price'], mode='markers',
        marker=dict(symbol='triangle-down', size=10, color='red'),
        name='Sell Orders',
    ), row=1, col=1)

    stock_pairs = build_trade_pairs(df_orders)
    if stock_pairs:
        add_trade_pair_connectors(fig, stock_pairs, row=1)
    else:
        fig.add_trace(go.Scatter(
            x=df_orders['Date'], y=df_orders['Price'],
            mode='lines', line=dict(dash='dot', color='black'),
            name='Buy-Sell Line',
        ), row=1, col=1)

    if strike_price_a:
        fig.add_trace(go.Scatter(
            x=[df['Date'].min(), df['Date'].max()],
            y=[strike_price_a, strike_price_a],
            mode='lines', line=dict(color='orange', width=2), name='Strike A',
        ), row=1, col=1)

    if strike_price_b and strike_price_b != strike_price_a:
        fig.add_trace(go.Scatter(
            x=[df['Date'].min(), df['Date'].max()],
            y=[strike_price_b, strike_price_b],
            mode='lines', line=dict(color='red', width=2), name='Strike B',
        ), row=1, col=1)

    # ------------------------------------------------------------------ #
    # "All Symbols" group: row-1 underlying markers + row-2 option traces
    # ------------------------------------------------------------------ #
    always_end = len(fig.data)   # end of row-1 always-visible traces

    # Row 1: underlying price at each option open/close (all symbols combined)
    all_und_df = _build_underlying_markers(df_option_orders, df)
    if not all_und_df.empty:
        open_und  = all_und_df[all_und_df['Type'] == 'Sell']
        close_und = all_und_df[all_und_df['Type'] == 'Buy']
        fig.add_trace(go.Scatter(
            x=open_und['Date'], y=open_und['Price'], mode='markers',
            marker=dict(symbol='diamond', size=9, color='orange'),
            name='Option Opened',
            customdata=_und_customdata(open_und),
            hovertemplate=_UND_OPT_HOVER,
        ), row=1, col=1)
        fig.add_trace(go.Scatter(
            x=close_und['Date'], y=close_und['Price'], mode='markers',
            marker=dict(symbol='diamond-open', size=9, color='purple', line=dict(width=2)),
            name='Option Closed',
            customdata=_und_customdata(close_und),
            hovertemplate=_UND_OPT_HOVER,
        ), row=1, col=1)
        und_pairs = build_trade_pairs(all_und_df)
        if und_pairs:
            add_trade_pair_connectors(fig, und_pairs, row=1)

    buy_opt  = df_option_orders[df_option_orders['Type'] == 'Buy']
    sell_opt = df_option_orders[df_option_orders['Type'] == 'Sell']

    fig.add_trace(go.Scatter(
        x=buy_opt['Date'], y=buy_opt['Price'], mode='markers',
        marker=dict(symbol='triangle-up', size=10, color='blue'),
        name='Buy Options',
        customdata=_opt_customdata(buy_opt),
        hovertemplate=_OPT_HOVER,
    ), row=2, col=1)

    fig.add_trace(go.Scatter(
        x=sell_opt['Date'], y=sell_opt['Price'], mode='markers',
        marker=dict(symbol='triangle-down', size=10, color='red'),
        name='Sell Options',
        customdata=_opt_customdata(sell_opt),
        hovertemplate=_OPT_HOVER,
    ), row=2, col=1)

    option_pairs = build_trade_pairs(df_option_orders)
    if option_pairs:
        add_trade_pair_connectors(fig, option_pairs, row=2)
    else:
        fig.add_trace(go.Scatter(
            x=df_option_orders['Date'], y=df_option_orders['Price'],
            mode='lines', line=dict(dash='dot', color='black'),
            name='Buy-Sell Line',
        ), row=2, col=1)

    all_sym_end = len(fig.data)   # end of "All Symbols" traces

    # ------------------------------------------------------------------ #
    # Row 2: per-symbol order traces (all hidden initially)
    # ------------------------------------------------------------------ #
    # Determine symbol list and default selection
    symbols_with_orders = (
        list(option_order_data_by_symbol.keys())
        if option_order_data_by_symbol
        else []
    )
    default_sym_idx = (
        symbols_with_orders.index(default_option_symbol)
        if default_option_symbol in symbols_with_orders
        else 0
    )

    sym_trace_ranges: List[tuple] = []   # (start, end) per symbol

    for sym in symbols_with_orders:
        sym_data = (option_order_data_by_symbol or {}).get(sym, {})
        df_sym = order_to_np(sym_data)
        start = len(fig.data)

        # Row 1: underlying price at each option open/close for this symbol
        sym_und_df = _build_underlying_markers(df_sym, df)
        if not sym_und_df.empty:
            sym_open_und  = sym_und_df[sym_und_df['Type'] == 'Sell']
            sym_close_und = sym_und_df[sym_und_df['Type'] == 'Buy']
            fig.add_trace(go.Scatter(
                x=sym_open_und['Date'], y=sym_open_und['Price'], mode='markers',
                marker=dict(symbol='diamond', size=9, color='orange'),
                name='Option Opened', showlegend=False, visible=False,
                customdata=_und_customdata(sym_open_und),
                hovertemplate=_UND_OPT_HOVER,
            ), row=1, col=1)
            fig.add_trace(go.Scatter(
                x=sym_close_und['Date'], y=sym_close_und['Price'], mode='markers',
                marker=dict(symbol='diamond-open', size=9, color='purple', line=dict(width=2)),
                name='Option Closed', showlegend=False, visible=False,
                customdata=_und_customdata(sym_close_und),
                hovertemplate=_UND_OPT_HOVER,
            ), row=1, col=1)
            sym_und_pairs = build_trade_pairs(sym_und_df)
            if sym_und_pairs:
                add_trade_pair_connectors(fig, sym_und_pairs, row=1)

        buy_s  = df_sym[df_sym['Type'] == 'Buy']
        sell_s = df_sym[df_sym['Type'] == 'Sell']

        fig.add_trace(go.Scatter(
            x=buy_s['Date'], y=buy_s['Price'], mode='markers',
            marker=dict(symbol='triangle-up', size=10, color='blue'),
            name='Buy Options', showlegend=False, visible=False,
            customdata=_opt_customdata(buy_s),
            hovertemplate=_OPT_HOVER,
        ), row=2, col=1)
        fig.add_trace(go.Scatter(
            x=sell_s['Date'], y=sell_s['Price'], mode='markers',
            marker=dict(symbol='triangle-down', size=10, color='red'),
            name='Sell Options', showlegend=False, visible=False,
            customdata=_opt_customdata(sell_s),
            hovertemplate=_OPT_HOVER,
        ), row=2, col=1)

        sym_pairs = build_trade_pairs(df_sym)
        if sym_pairs:
            add_trade_pair_connectors(fig, sym_pairs, row=2)
        else:
            fig.add_trace(go.Scatter(
                x=df_sym['Date'], y=df_sym['Price'],
                mode='lines', line=dict(dash='dot', color='black'),
                name='Buy-Sell Line', showlegend=False, visible=False,
            ), row=2, col=1)

        end = len(fig.data)
        sym_trace_ranges.append((start, end))
        # Hide all per-symbol traces we just added
        for j in range(start, end):
            fig.data[j].update(visible=False)

    # ------------------------------------------------------------------ #
    # Dropdown: switch between "All Symbols" and per-symbol order views
    # ------------------------------------------------------------------ #
    total_traces = len(fig.data)
    row1_indices  = set(range(0, always_end))
    all_sym_set   = set(range(always_end, all_sym_end))
    per_sym_sets  = [set(range(s, e)) for s, e in sym_trace_ranges]
    all_per_sym   = set().union(*per_sym_sets) if per_sym_sets else set()

    def _vis_for(active_set: set) -> List[bool]:
        return [
            True if j in row1_indices
            else True if j in active_set
            else False
            for j in range(total_traces)
        ]

    dropdown_buttons = [dict(
        label='All Symbols',
        method='update',
        args=[{'visible': _vis_for(all_sym_set)},
              {'title': chart_title}],
    )]
    for i, sym in enumerate(symbols_with_orders):
        label = _short_option_label(sym)
        dropdown_buttons.append(dict(
            label=label,
            method='update',
            args=[{'visible': _vis_for(per_sym_sets[i])},
                  {'title': f"{chart_title}<br><sup>{sym}</sup>"}],
        ))

    default_active = 0 if not symbols_with_orders else default_sym_idx + 1

    fig.update_layout(
        title=chart_title,
        yaxis_title='Underlying Price',
        xaxis2_title='Date',
        yaxis2_title='Option Price',
        xaxis_rangeslider_visible=False,
        legend=dict(x=1.02, xanchor='left', y=1, yanchor='top'),
        updatemenus=[dict(
            type='dropdown',
            buttons=dropdown_buttons,
            direction='down',
            showactive=True,
            active=0,   # "All Symbols" selected initially
            x=0.0,
            xanchor='left',
            y=1.0,
            yanchor='bottom',
            bgcolor='lightyellow',
            bordercolor='#333333',
            borderwidth=2,
            font=dict(size=13, color='black'),
        )] if dropdown_buttons else [],
        margin=dict(r=220, t=120),
    )
    import os
    out_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'chart_output.html')
    fig.write_html(out_path, auto_open=True)
    print(f"Chart saved to: {out_path}", file=sys.stderr)


# ------------------------------------------------------------------ #
# Data conversion helpers (used by both fetch and legacy paths)
# ------------------------------------------------------------------ #

def _parse_dates(dates) -> pd.Series:
    """
    Parse a list of date strings robustly.

    - Strips trailing alphabetic timezone abbreviations (e.g. 'EST', 'EDT')
      that pd.to_datetime cannot handle.
    - Uses errors='coerce' so malformed/zero-value proto timestamps (e.g.
      year 56976) become NaT instead of raising.
    """
    cleaned = []
    for d in dates:
        if isinstance(d, str):
            parts = d.rsplit(' ', 1)
            if len(parts) == 2 and parts[1].isalpha():
                d = parts[0]
        cleaned.append(d)
    return pd.to_datetime(cleaned, utc=True, errors='coerce', format='mixed')


def option_to_np(data: Dict[str, Any]) -> pd.DataFrame:
    if not data.get('Date'):
        return pd.DataFrame(columns=['Date', 'Open', 'Close'])
    df = pd.DataFrame()
    df['Date']  = _parse_dates(data['Date'])
    df['Open']  = np.array(data['Open'])
    df['Close'] = np.array(data['Close'])
    df = df.dropna(subset=['Date'])
    return df


def order_to_np(data: Dict[str, Any]) -> pd.DataFrame:
    df = pd.DataFrame()
    df['Date']  = _parse_dates(data.get('Date', []))
    df['Price'] = np.array(data.get('Price', []))
    df['Type']  = np.array(data.get('Type', []))
    if 'PairId' in data:
        df['PairId'] = np.array(data['PairId'], dtype=object)
    if 'PnL' in data:
        df['PnL'] = pd.to_numeric(data['PnL'], errors='coerce')
    if 'Symbol' in data:
        df['Symbol'] = np.array(data['Symbol'], dtype=object)
    return df


def candle_to_np(data: Dict[str, Any], timeframeInMinutes: int) -> pd.DataFrame:
    df = pd.DataFrame()
    df['Date']  = _parse_dates(data['Date'])
    df['Open']  = np.array(data['Open'])
    df['High']  = np.array(data['High'])
    df['Low']   = np.array(data['Low'])
    df['Close'] = np.array(data['Close'])
    return df


# ------------------------------------------------------------------ #
# Entry point
# ------------------------------------------------------------------ #

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Plot candlestick chart with option trade overlays.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument('--playground-id', type=str,
                        help='Playground UUID to fetch data from')
    parser.add_argument('--twirp-url', type=str, default='http://127.0.0.1:5051',
                        help='Twirp server URL (default: http://127.0.0.1:5051)')
    parser.add_argument('--symbol', type=str, default='AAPL',
                        help='Underlying symbol to plot (used with --playground-id)')
    parser.add_argument('--timeframe', type=int, default=3600,
                        help='Candle period in seconds (default: 3600)')
    parser.add_argument('--option-symbol', type=str, default=None,
                        help='Option symbol for price-line in subplot 2 (e.g. O:AAPL250103P00245000). '
                             'If omitted, first traded symbol is used. Run without this flag to see available symbols.')
    parser.add_argument('--json', type=str, dest='json_input',
                        help='Input data as a JSON string (legacy mode)')
    # Legacy: bare positional JSON blob kept for backward compatibility
    parser.add_argument('json_positional', nargs='?', type=str,
                        help=argparse.SUPPRESS)

    args = parser.parse_args()

    if args.playground_id:
        input_data = fetch_playground_data(
            playground_id=args.playground_id,
            symbol=args.symbol,
            timeframe=args.timeframe,
            twirp_url=args.twirp_url,
            option_symbol=args.option_symbol,
        )
    elif args.json_input or args.json_positional:
        input_data = json.loads(args.json_input or args.json_positional)
    else:
        parser.error("Provide --playground-id or --json '<json>'")
        return

    chart_data        = input_data['chart_data']
    candle_data       = input_data['candle_data']
    order_data        = input_data['order_data']
    option_data       = input_data['option_data']
    option_order_data = input_data['option_order_data']

    plot_candlestick(
        chart_data['title'],
        chart_data['subplot_1_title'],
        chart_data['subplot_2_title'],
        candle_data, order_data, option_data, option_order_data,
        chart_data['timeframe'],
        option_data_by_symbol=input_data.get('option_data_by_symbol'),
        default_option_symbol=chart_data.get('default_option_symbol'),
        option_order_data_by_symbol=input_data.get('option_order_data_by_symbol'),
    )


if __name__ == "__main__":
    main()
