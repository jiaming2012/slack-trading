// JS Example for subscribing to a channel
/* eslint-disable */
const WebSocket = require('ws');
const CryptoJS = require('crypto-js');
const fs = require('fs');

// Derived from your Coinbase Retail API Key
//  SIGNING_KEY: the signing key provided as a part of your API key. Also called the "SECRET KEY"
//  API_KEY: the tradeapi key provided as a part of your API key. also called the "PUBLIC KEY"
const SIGNING_KEY = 's2RceoHWEaLYxnaeOUm2tpmNLsELkaGy';
const API_KEY = 'UPveTLyBzHNzsXRw';

if (!SIGNING_KEY.length || !API_KEY.length) {
    throw new Error('missing mandatory environment variable(s)');
}

const CHANNEL_NAMES = {
    level2: 'level2',
    user: 'user',
    tickers: 'ticker',
    ticker_batch: 'ticker_batch',
    status: 'status',
    market_trades: 'market_trades',
};

// The base URL of the API
const WS_API_URL = 'wss://advanced-trade-ws.coinbase.com';

// Function to generate a signature using CryptoJS
function sign(str, secret) {
    const hash = CryptoJS.HmacSHA256(str, secret);
    return hash.toString();
}

function timestampAndSign(message, channel, products = []) {
    // const timestamp = Math.floor(Date.now() / 1000).toString();
    const timestamp = '1680318126'
    const strToSign = `${timestamp}${channel}${products.join(',')}`;
    console.log('strToSign: ', strToSign);
    const sig = sign(strToSign, SIGNING_KEY);
    console.log('sig: ', sig)
    return { ...message, signature: sig, timestamp: timestamp };
}

const ws = new WebSocket(WS_API_URL);

function subscribeToProducts(products, channelName, ws) {
    const message = {
        type: 'subscribe',
        channel: channelName,
        api_key: API_KEY,
        product_ids: products,
    };
    const subscribeMsg = timestampAndSign(message, channelName, products);
    console.log('msg: ', subscribeMsg);
    ws.send(JSON.stringify(subscribeMsg));
}

function unsubscribeToProducts(products, channelName, ws) {
    const message = {
        type: 'unsubscribe',
        channel: channelName,
        api_key: API_KEY,
        product_ids: products,
    };
    const subscribeMsg = timestampAndSign(message, channelName, products);
    ws.send(JSON.stringify(subscribeMsg));
}

function onMessage(data) {
    const parsedData = JSON.parse(data);
    console.log(parsedData)
}

const connections = [];
let sentUnsub = false;
for (let i = 0; i < 1; i++) {
    const date1 = new Date(new Date().toUTCString());
    const ws = new WebSocket(WS_API_URL);

    ws.on('message', function (data) {
        // const date2 = new Date(new Date().toUTCString());
        // const diffTime = Math.abs(date2 - date1);
        // if (diffTime > 5000 && !sentUnsub) {
        //     unsubscribeToProducts(['BTC-USD'], CHANNEL_NAMES.tickers, ws);
        //     sentUnsub = true;
        // }

        const parsedData = JSON.parse(data);
        console.log(JSON.stringify(parsedData, null, 4));
    });

    ws.on('open', function () {
        const products = ['BTC-USD'];
        subscribeToProducts(products, CHANNEL_NAMES.tickers, ws);
    });

    connections.push(ws);
}