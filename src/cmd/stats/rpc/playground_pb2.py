# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: playground.proto
# Protobuf Python Version: 5.29.3
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC,
    5,
    29,
    3,
    '',
    'playground.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2
from google.protobuf import duration_pb2 as google_dot_protobuf_dot_duration__pb2
from google.protobuf import empty_pb2 as google_dot_protobuf_dot_empty__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x10playground.proto\x12\nplayground\x1a\x1fgoogle/protobuf/timestamp.proto\x1a\x1egoogle/protobuf/duration.proto\x1a\x1bgoogle/protobuf/empty.proto\"W\n\x14MockFillOrderRequest\x12\x10\n\x08order_id\x18\x01 \x01(\x04\x12\r\n\x05price\x18\x02 \x01(\x01\x12\x0e\n\x06status\x18\x03 \x01(\t\x12\x0e\n\x06\x62roker\x18\x04 \x01(\t\"A\n\x1eGetReconciliationReportRequest\x12\x1f\n\x17reconcile_playground_id\x18\x01 \x01(\t\"`\n\x0ePositionReport\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x10\n\x08quantity\x18\x02 \x01(\x01\x12\x1a\n\rplayground_id\x18\x03 \x01(\tH\x00\x88\x01\x01\x42\x10\n\x0e_playground_id\"\xd4\x01\n\x1fGetReconciliationReportResponse\x12\x34\n\x10\x62roker_positions\x18\x01 \x03(\x0b\x32\x1a.playground.PositionReport\x12=\n\x19live_playground_positions\x18\x02 \x03(\x0b\x32\x1a.playground.PositionReport\x12<\n\x18reconciliation_positions\x18\x03 \x03(\x0b\x32\x1a.playground.PositionReport\"%\n\x15GetPlaygroundsRequest\x12\x0c\n\x04tags\x18\x01 \x03(\t\"(\n\x15GetAppVersionResponse\x12\x0f\n\x07version\x18\x01 \x01(\t\"\xc7\x02\n\x0b\x41\x63\x63ountMeta\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12$\n\x17reconcile_playground_id\x18\x02 \x01(\tH\x00\x88\x01\x01\x12\x12\n\nstart_date\x18\x03 \x01(\t\x12\x15\n\x08\x65nd_date\x18\x04 \x01(\tH\x01\x88\x01\x01\x12\x0f\n\x07symbols\x18\x05 \x03(\t\x12\x17\n\x0finitial_balance\x18\x06 \x01(\x01\x12\x13\n\x0b\x65nvironment\x18\x07 \x01(\t\x12\x1e\n\x11live_account_type\x18\x08 \x01(\tH\x02\x88\x01\x01\x12\x0c\n\x04tags\x18\t \x03(\t\x12\x16\n\tclient_id\x18\n \x01(\tH\x03\x88\x01\x01\x42\x1a\n\x18_reconcile_playground_idB\x0b\n\t_end_dateB\x14\n\x12_live_account_typeB\x0c\n\n_client_id\"D\n\x16GetAccountStatsRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x13\n\x0b\x65quity_plot\x18\x02 \x01(\x08\"0\n\nEquityPlot\x12\x12\n\ncreated_at\x18\x01 \x01(\t\x12\x0e\n\x06\x65quity\x18\x02 \x01(\x01\"F\n\x17GetAccountStatsResponse\x12+\n\x0b\x65quity_plot\x18\x01 \x03(\x0b\x32\x16.playground.EquityPlot\"\xe0\x02\n\x11PlaygroundSession\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12%\n\x04meta\x18\x02 \x01(\x0b\x32\x17.playground.AccountMeta\x12 \n\x05\x63lock\x18\x03 \x01(\x0b\x32\x11.playground.Clock\x12,\n\x0crepositories\x18\x04 \x03(\x0b\x32\x16.playground.Repository\x12\x0f\n\x07\x62\x61lance\x18\x05 \x01(\x01\x12\x0e\n\x06\x65quity\x18\x06 \x01(\x01\x12\x13\n\x0b\x66ree_margin\x18\x07 \x01(\x01\x12?\n\tpositions\x18\x08 \x03(\x0b\x32,.playground.PlaygroundSession.PositionsEntry\x1a\x46\n\x0ePositionsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12#\n\x05value\x18\x02 \x01(\x0b\x32\x14.playground.Position:\x02\x38\x01\"L\n\x16GetPlaygroundsResponse\x12\x32\n\x0bplaygrounds\x18\x01 \x03(\x0b\x32\x1d.playground.PlaygroundSession\":\n\x15GetOpenOrdersResponse\x12!\n\x06orders\x18\x01 \x03(\x0b\x32\x11.playground.Order\"=\n\x14GetOpenOrdersRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\".\n\x15SavePlaygroundRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\"\x0f\n\rEmptyResponse\"H\n\x05\x43lock\x12\x14\n\x0c\x63urrent_time\x18\x01 \x01(\t\x12\r\n\x05start\x18\x02 \x01(\t\x12\x11\n\x04stop\x18\x03 \x01(\tH\x00\x88\x01\x01\x42\x07\n\x05_stop\"}\n\nRepository\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x1b\n\x13timespan_multiplier\x18\x02 \x01(\r\x12\x15\n\rtimespan_unit\x18\x03 \x01(\t\x12\x12\n\nindicators\x18\x04 \x03(\t\x12\x17\n\x0fhistory_in_days\x18\x05 \x01(\r\"\xcf\x01\n\x1e\x43reatePolygonPlaygroundRequest\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x01\x12\x12\n\nstart_date\x18\x02 \x01(\t\x12\x11\n\tstop_date\x18\x03 \x01(\t\x12,\n\x0crepositories\x18\x04 \x03(\x0b\x32\x16.playground.Repository\x12\x13\n\x0b\x65nvironment\x18\x05 \x01(\t\x12\x16\n\tclient_id\x18\x06 \x01(\tH\x00\x88\x01\x01\x12\x0c\n\x04tags\x18\x07 \x03(\tB\x0c\n\n_client_id\"0\n\x17\x44\x65letePlaygroundRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\"\xcb\x01\n\x1b\x43reateLivePlaygroundRequest\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x01\x12\x0e\n\x06\x62roker\x18\x02 \x01(\t\x12,\n\x0crepositories\x18\x03 \x03(\x0b\x32\x16.playground.Repository\x12\x13\n\x0b\x65nvironment\x18\x04 \x01(\t\x12\x14\n\x0c\x61\x63\x63ount_type\x18\x05 \x01(\t\x12\x16\n\tclient_id\x18\x06 \x01(\tH\x00\x88\x01\x01\x12\x0c\n\x04tags\x18\x07 \x03(\tB\x0c\n\n_client_id\"}\n\x11GetCandlesRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x19\n\x11period_in_seconds\x18\x03 \x01(\x05\x12\x13\n\x0b\x66romRTF3339\x18\x04 \x01(\t\x12\x11\n\ttoRTF3339\x18\x05 \x01(\t\"3\n\x12GetCandlesResponse\x12\x1d\n\x04\x62\x61rs\x18\x01 \x03(\x0b\x32\x0f.playground.Bar\"&\n\x18\x43reatePlaygroundResponse\x12\n\n\x02id\x18\x01 \x01(\t\"\xc0\x01\n\x11GetAccountRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x14\n\x0c\x66\x65tch_orders\x18\x02 \x01(\x08\x12\x18\n\x0b\x66romRTF3339\x18\x03 \x01(\tH\x00\x88\x01\x01\x12\x16\n\ttoRTF3339\x18\x04 \x01(\tH\x01\x88\x01\x01\x12\x0e\n\x06status\x18\x05 \x03(\t\x12\r\n\x05sides\x18\x06 \x03(\t\x12\x0f\n\x07symbols\x18\x07 \x03(\tB\x0e\n\x0c_fromRTF3339B\x0c\n\n_toRTF3339\"\x9e\x02\n\x12GetAccountResponse\x12%\n\x04meta\x18\x01 \x01(\x0b\x32\x17.playground.AccountMeta\x12\x0f\n\x07\x62\x61lance\x18\x02 \x01(\x01\x12\x0e\n\x06\x65quity\x18\x03 \x01(\x01\x12\x13\n\x0b\x66ree_margin\x18\x04 \x01(\x01\x12!\n\x06orders\x18\x05 \x03(\x0b\x32\x11.playground.Order\x12@\n\tpositions\x18\x06 \x03(\x0b\x32-.playground.GetAccountResponse.PositionsEntry\x1a\x46\n\x0ePositionsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12#\n\x05value\x18\x02 \x01(\x0b\x32\x14.playground.Position:\x02\x38\x01\"o\n\x08Position\x12\x10\n\x08quantity\x18\x01 \x01(\x01\x12\x12\n\ncost_basis\x18\x02 \x01(\x01\x12\n\n\x02pl\x18\x03 \x01(\x01\x12\x1a\n\x12maintenance_margin\x18\x04 \x01(\x01\x12\x15\n\rcurrent_price\x18\x05 \x01(\x01\"\xd0\x02\n\x11PlaceOrderRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x13\n\x0b\x61sset_class\x18\x03 \x01(\t\x12\x10\n\x08quantity\x18\x04 \x01(\x01\x12\x0c\n\x04side\x18\x05 \x01(\t\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\x12\x0b\n\x03tag\x18\x08 \x01(\t\x12\x17\n\x0frequested_price\x18\t \x01(\x01\x12\x12\n\x05price\x18\n \x01(\x01H\x00\x88\x01\x01\x12\x1b\n\x0e\x63lose_order_id\x18\x0b \x01(\x04H\x01\x88\x01\x01\x12\x15\n\ris_adjustment\x18\x0c \x01(\x08\x12\x1e\n\x11\x63lient_request_id\x18\r \x01(\tH\x02\x88\x01\x01\x42\x08\n\x06_priceB\x11\n\x0f_close_order_idB\x14\n\x12_client_request_id\"a\n\x0fNextTickRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0f\n\x07seconds\x18\x02 \x01(\x04\x12\x12\n\nis_preview\x18\x03 \x01(\x08\x12\x12\n\nrequest_id\x18\x04 \x01(\t\"\xe6\x01\n\tTickDelta\x12%\n\nnew_trades\x18\x01 \x03(\x0b\x32\x11.playground.Trade\x12\'\n\x0bnew_candles\x18\x02 \x03(\x0b\x32\x12.playground.Candle\x12)\n\x0einvalid_orders\x18\x03 \x03(\x0b\x32\x11.playground.Order\x12*\n\x06\x65vents\x18\x04 \x03(\x0b\x32\x1a.playground.TickDeltaEvent\x12\x14\n\x0c\x63urrent_time\x18\x05 \x01(\t\x12\x1c\n\x14is_backtest_complete\x18\x06 \x01(\x08\"<\n\x10LiquidationEvent\x12(\n\rorders_placed\x18\x01 \x03(\x0b\x32\x11.playground.Order\"W\n\x0eTickDeltaEvent\x12\x0c\n\x04type\x18\x01 \x01(\t\x12\x37\n\x11liquidation_event\x18\x02 \x01(\x0b\x32\x1c.playground.LiquidationEvent\"\xcf\x06\n\x03\x42\x61r\x12\x0e\n\x06volume\x18\x01 \x01(\x01\x12\x0c\n\x04open\x18\x02 \x01(\x01\x12\r\n\x05\x63lose\x18\x03 \x01(\x01\x12\x0c\n\x04high\x18\x04 \x01(\x01\x12\x0b\n\x03low\x18\x05 \x01(\x01\x12\x10\n\x08\x64\x61tetime\x18\x06 \x01(\t\x12\x13\n\x0bsuperT_50_3\x18\x07 \x01(\x01\x12\x13\n\x0bsuperD_50_3\x18\x08 \x01(\x05\x12\x13\n\x0bsuperL_50_3\x18\t \x01(\x01\x12\x13\n\x0bsuperS_50_3\x18\n \x01(\x01\x12\x1c\n\x14stochrsi_k_14_14_3_3\x18\x0b \x01(\x01\x12\x1c\n\x14stochrsi_d_14_14_3_3\x18\x0c \x01(\x01\x12\x0e\n\x06\x61tr_14\x18\r \x01(\x01\x12\x0e\n\x06sma_50\x18\x0e \x01(\x01\x12\x0f\n\x07sma_100\x18\x0f \x01(\x01\x12\x0f\n\x07sma_200\x18\x10 \x01(\x01\x12\x1f\n\x17stochrsi_cross_above_20\x18\x11 \x01(\x08\x12\x1f\n\x17stochrsi_cross_below_80\x18\x12 \x01(\x08\x12\x13\n\x0b\x63lose_lag_1\x18\x13 \x01(\x01\x12\x13\n\x0b\x63lose_lag_2\x18\x14 \x01(\x01\x12\x13\n\x0b\x63lose_lag_3\x18\x15 \x01(\x01\x12\x13\n\x0b\x63lose_lag_4\x18\x16 \x01(\x01\x12\x13\n\x0b\x63lose_lag_5\x18\x17 \x01(\x01\x12\x13\n\x0b\x63lose_lag_6\x18\x18 \x01(\x01\x12\x13\n\x0b\x63lose_lag_7\x18\x19 \x01(\x01\x12\x13\n\x0b\x63lose_lag_8\x18\x1a \x01(\x01\x12\x13\n\x0b\x63lose_lag_9\x18\x1b \x01(\x01\x12\x14\n\x0c\x63lose_lag_10\x18\x1c \x01(\x01\x12\x14\n\x0c\x63lose_lag_11\x18\x1d \x01(\x01\x12\x14\n\x0c\x63lose_lag_12\x18\x1e \x01(\x01\x12\x14\n\x0c\x63lose_lag_13\x18\x1f \x01(\x01\x12\x14\n\x0c\x63lose_lag_14\x18  \x01(\x01\x12\x14\n\x0c\x63lose_lag_15\x18! \x01(\x01\x12\x14\n\x0c\x63lose_lag_16\x18\" \x01(\x01\x12\x14\n\x0c\x63lose_lag_17\x18# \x01(\x01\x12\x14\n\x0c\x63lose_lag_18\x18$ \x01(\x01\x12\x14\n\x0c\x63lose_lag_19\x18% \x01(\x01\x12\x14\n\x0c\x63lose_lag_20\x18& \x01(\x01\x12\x12\n\ncdl_hammer\x18\' \x01(\x01\x12\x17\n\x0f\x63\x64l_doji_10_0_1\x18( \x01(\x01\"F\n\x06\x43\x61ndle\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06period\x18\x02 \x01(\x05\x12\x1c\n\x03\x62\x61r\x18\x03 \x01(\x0b\x32\x0f.playground.Bar\"\xa5\x01\n\x05Trade\x12\n\n\x02id\x18\x01 \x01(\x04\x12\x13\n\x0b\x63reate_date\x18\x02 \x01(\t\x12\x10\n\x08quantity\x18\x03 \x01(\x01\x12\r\n\x05price\x18\x04 \x01(\x01\x12\x15\n\x08order_id\x18\x05 \x01(\x04H\x00\x88\x01\x01\x12\x1f\n\x12reconcile_order_id\x18\x06 \x01(\x04H\x01\x88\x01\x01\x42\x0b\n\t_order_idB\x15\n\x13_reconcile_order_id\"\xea\x03\n\x05Order\x12\n\n\x02id\x18\x01 \x01(\x04\x12\r\n\x05\x63lass\x18\x02 \x01(\t\x12\x0e\n\x06symbol\x18\x03 \x01(\t\x12\x0c\n\x04side\x18\x04 \x01(\t\x12\x10\n\x08quantity\x18\x05 \x01(\x01\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\x12\r\n\x05price\x18\x08 \x01(\x01\x12\x17\n\x0frequested_price\x18\t \x01(\x01\x12\x12\n\nstop_price\x18\n \x01(\x01\x12\x0b\n\x03tag\x18\x0b \x01(\t\x12!\n\x06trades\x18\x0c \x03(\x0b\x32\x11.playground.Trade\x12\x0e\n\x06status\x18\r \x01(\t\x12\x15\n\rreject_reason\x18\x0e \x01(\t\x12\x13\n\x0b\x63reate_date\x18\x0f \x01(\t\x12$\n\tclosed_by\x18\x10 \x03(\x0b\x32\x11.playground.Trade\x12!\n\x06\x63loses\x18\x11 \x03(\x0b\x32\x11.playground.Order\x12\x18\n\x0b\x65xternal_id\x18\x12 \x01(\x04H\x00\x88\x01\x01\x12%\n\nreconciles\x18\x13 \x03(\x0b\x32\x11.playground.Order\x12\x1e\n\x11\x63lient_request_id\x18\x14 \x01(\tH\x01\x88\x01\x01\x42\x0e\n\x0c_external_idB\x14\n\x12_client_request_id2\xb7\t\n\x11PlaygroundService\x12\x64\n\x10\x43reatePlayground\x12*.playground.CreatePolygonPlaygroundRequest\x1a$.playground.CreatePlaygroundResponse\x12\x65\n\x14\x43reateLivePlayground\x12\'.playground.CreateLivePlaygroundRequest\x1a$.playground.CreatePlaygroundResponse\x12W\n\x0eGetPlaygrounds\x12!.playground.GetPlaygroundsRequest\x1a\".playground.GetPlaygroundsResponse\x12>\n\x08NextTick\x12\x1b.playground.NextTickRequest\x1a\x15.playground.TickDelta\x12>\n\nPlaceOrder\x12\x1d.playground.PlaceOrderRequest\x1a\x11.playground.Order\x12K\n\nGetAccount\x12\x1d.playground.GetAccountRequest\x1a\x1e.playground.GetAccountResponse\x12K\n\nGetCandles\x12\x1d.playground.GetCandlesRequest\x1a\x1e.playground.GetCandlesResponse\x12T\n\rGetOpenOrders\x12 .playground.GetOpenOrdersRequest\x1a!.playground.GetOpenOrdersResponse\x12N\n\x0eSavePlayground\x12!.playground.SavePlaygroundRequest\x1a\x19.playground.EmptyResponse\x12R\n\x10\x44\x65letePlayground\x12#.playground.DeletePlaygroundRequest\x1a\x19.playground.EmptyResponse\x12Z\n\x0fGetAccountStats\x12\".playground.GetAccountStatsRequest\x1a#.playground.GetAccountStatsResponse\x12J\n\rGetAppVersion\x12\x16.google.protobuf.Empty\x1a!.playground.GetAppVersionResponse\x12r\n\x17GetReconciliationReport\x12*.playground.GetReconciliationReportRequest\x1a+.playground.GetReconciliationReportResponse\x12L\n\rMockFillOrder\x12 .playground.MockFillOrderRequest\x1a\x19.playground.EmptyResponseB\x0eZ\x0c./playgroundb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'playground_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z\014./playground'
  _globals['_PLAYGROUNDSESSION_POSITIONSENTRY']._loaded_options = None
  _globals['_PLAYGROUNDSESSION_POSITIONSENTRY']._serialized_options = b'8\001'
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._loaded_options = None
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_options = b'8\001'
  _globals['_MOCKFILLORDERREQUEST']._serialized_start=126
  _globals['_MOCKFILLORDERREQUEST']._serialized_end=213
  _globals['_GETRECONCILIATIONREPORTREQUEST']._serialized_start=215
  _globals['_GETRECONCILIATIONREPORTREQUEST']._serialized_end=280
  _globals['_POSITIONREPORT']._serialized_start=282
  _globals['_POSITIONREPORT']._serialized_end=378
  _globals['_GETRECONCILIATIONREPORTRESPONSE']._serialized_start=381
  _globals['_GETRECONCILIATIONREPORTRESPONSE']._serialized_end=593
  _globals['_GETPLAYGROUNDSREQUEST']._serialized_start=595
  _globals['_GETPLAYGROUNDSREQUEST']._serialized_end=632
  _globals['_GETAPPVERSIONRESPONSE']._serialized_start=634
  _globals['_GETAPPVERSIONRESPONSE']._serialized_end=674
  _globals['_ACCOUNTMETA']._serialized_start=677
  _globals['_ACCOUNTMETA']._serialized_end=1004
  _globals['_GETACCOUNTSTATSREQUEST']._serialized_start=1006
  _globals['_GETACCOUNTSTATSREQUEST']._serialized_end=1074
  _globals['_EQUITYPLOT']._serialized_start=1076
  _globals['_EQUITYPLOT']._serialized_end=1124
  _globals['_GETACCOUNTSTATSRESPONSE']._serialized_start=1126
  _globals['_GETACCOUNTSTATSRESPONSE']._serialized_end=1196
  _globals['_PLAYGROUNDSESSION']._serialized_start=1199
  _globals['_PLAYGROUNDSESSION']._serialized_end=1551
  _globals['_PLAYGROUNDSESSION_POSITIONSENTRY']._serialized_start=1481
  _globals['_PLAYGROUNDSESSION_POSITIONSENTRY']._serialized_end=1551
  _globals['_GETPLAYGROUNDSRESPONSE']._serialized_start=1553
  _globals['_GETPLAYGROUNDSRESPONSE']._serialized_end=1629
  _globals['_GETOPENORDERSRESPONSE']._serialized_start=1631
  _globals['_GETOPENORDERSRESPONSE']._serialized_end=1689
  _globals['_GETOPENORDERSREQUEST']._serialized_start=1691
  _globals['_GETOPENORDERSREQUEST']._serialized_end=1752
  _globals['_SAVEPLAYGROUNDREQUEST']._serialized_start=1754
  _globals['_SAVEPLAYGROUNDREQUEST']._serialized_end=1800
  _globals['_EMPTYRESPONSE']._serialized_start=1802
  _globals['_EMPTYRESPONSE']._serialized_end=1817
  _globals['_CLOCK']._serialized_start=1819
  _globals['_CLOCK']._serialized_end=1891
  _globals['_REPOSITORY']._serialized_start=1893
  _globals['_REPOSITORY']._serialized_end=2018
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_start=2021
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_end=2228
  _globals['_DELETEPLAYGROUNDREQUEST']._serialized_start=2230
  _globals['_DELETEPLAYGROUNDREQUEST']._serialized_end=2278
  _globals['_CREATELIVEPLAYGROUNDREQUEST']._serialized_start=2281
  _globals['_CREATELIVEPLAYGROUNDREQUEST']._serialized_end=2484
  _globals['_GETCANDLESREQUEST']._serialized_start=2486
  _globals['_GETCANDLESREQUEST']._serialized_end=2611
  _globals['_GETCANDLESRESPONSE']._serialized_start=2613
  _globals['_GETCANDLESRESPONSE']._serialized_end=2664
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_start=2666
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_end=2704
  _globals['_GETACCOUNTREQUEST']._serialized_start=2707
  _globals['_GETACCOUNTREQUEST']._serialized_end=2899
  _globals['_GETACCOUNTRESPONSE']._serialized_start=2902
  _globals['_GETACCOUNTRESPONSE']._serialized_end=3188
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_start=1481
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_end=1551
  _globals['_POSITION']._serialized_start=3190
  _globals['_POSITION']._serialized_end=3301
  _globals['_PLACEORDERREQUEST']._serialized_start=3304
  _globals['_PLACEORDERREQUEST']._serialized_end=3640
  _globals['_NEXTTICKREQUEST']._serialized_start=3642
  _globals['_NEXTTICKREQUEST']._serialized_end=3739
  _globals['_TICKDELTA']._serialized_start=3742
  _globals['_TICKDELTA']._serialized_end=3972
  _globals['_LIQUIDATIONEVENT']._serialized_start=3974
  _globals['_LIQUIDATIONEVENT']._serialized_end=4034
  _globals['_TICKDELTAEVENT']._serialized_start=4036
  _globals['_TICKDELTAEVENT']._serialized_end=4123
  _globals['_BAR']._serialized_start=4126
  _globals['_BAR']._serialized_end=4973
  _globals['_CANDLE']._serialized_start=4975
  _globals['_CANDLE']._serialized_end=5045
  _globals['_TRADE']._serialized_start=5048
  _globals['_TRADE']._serialized_end=5213
  _globals['_ORDER']._serialized_start=5216
  _globals['_ORDER']._serialized_end=5706
  _globals['_PLAYGROUNDSERVICE']._serialized_start=5709
  _globals['_PLAYGROUNDSERVICE']._serialized_end=6916
# @@protoc_insertion_point(module_scope)
