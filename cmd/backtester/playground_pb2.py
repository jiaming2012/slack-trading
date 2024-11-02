# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: playground.proto
# Protobuf Python Version: 5.27.2
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC,
    5,
    27,
    2,
    '',
    'playground.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x10playground.proto\x12\nplayground\"\x9c\x01\n\x1e\x43reatePolygonPlaygroundRequest\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x02\x12\x12\n\nstart_date\x18\x02 \x01(\t\x12\x11\n\tstop_date\x18\x03 \x01(\t\x12\x0e\n\x06symbol\x18\x04 \x01(\t\x12\x1b\n\x13timespan_multiplier\x18\x05 \x01(\r\x12\x15\n\rtimespan_unit\x18\x06 \x01(\t\"b\n\x11GetCandlesRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x13\n\x0b\x66romRTF3339\x18\x03 \x01(\t\x12\x11\n\ttoRTF3339\x18\x04 \x01(\t\"3\n\x12GetCandlesResponse\x12\x1d\n\x04\x62\x61rs\x18\x01 \x03(\x0b\x32\x0f.playground.Bar\"&\n\x18\x43reatePlaygroundResponse\x12\n\n\x02id\x18\x01 \x01(\t\"@\n\x11GetAccountRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x14\n\x0c\x66\x65tch_orders\x18\x02 \x01(\x08\"\xf7\x01\n\x12GetAccountResponse\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x02\x12\x0e\n\x06\x65quity\x18\x02 \x01(\x02\x12\x13\n\x0b\x66ree_margin\x18\x03 \x01(\x02\x12!\n\x06orders\x18\x04 \x03(\x0b\x32\x11.playground.Order\x12@\n\tpositions\x18\x05 \x03(\x0b\x32-.playground.GetAccountResponse.PositionsEntry\x1a\x46\n\x0ePositionsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12#\n\x05value\x18\x02 \x01(\x0b\x32\x14.playground.Position:\x02\x38\x01\"X\n\x08Position\x12\x10\n\x08quantity\x18\x01 \x01(\x02\x12\x12\n\ncost_basis\x18\x02 \x01(\x01\x12\n\n\x02pl\x18\x03 \x01(\x01\x12\x1a\n\x12maintenance_margin\x18\x04 \x01(\x02\"\x8f\x01\n\x11PlaceOrderRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x13\n\x0b\x61sset_class\x18\x03 \x01(\t\x12\x10\n\x08quantity\x18\x04 \x01(\x01\x12\x0c\n\x04side\x18\x05 \x01(\t\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\"M\n\x0fNextTickRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0f\n\x07seconds\x18\x02 \x01(\x04\x12\x12\n\nis_preview\x18\x03 \x01(\x08\"\xe6\x01\n\tTickDelta\x12%\n\nnew_trades\x18\x01 \x03(\x0b\x32\x11.playground.Trade\x12\'\n\x0bnew_candles\x18\x02 \x03(\x0b\x32\x12.playground.Candle\x12)\n\x0einvalid_orders\x18\x03 \x03(\x0b\x32\x11.playground.Order\x12*\n\x06\x65vents\x18\x04 \x03(\x0b\x32\x1a.playground.TickDeltaEvent\x12\x14\n\x0c\x63urrent_time\x18\x05 \x01(\t\x12\x1c\n\x14is_backtest_complete\x18\x06 \x01(\x08\"<\n\x10LiquidationEvent\x12(\n\rorders_placed\x18\x01 \x03(\x0b\x32\x11.playground.Order\"W\n\x0eTickDeltaEvent\x12\x0c\n\x04type\x18\x01 \x01(\t\x12\x37\n\x11liquidation_event\x18\x02 \x01(\x0b\x32\x1c.playground.LiquidationEvent\"_\n\x03\x42\x61r\x12\x0e\n\x06volume\x18\x01 \x01(\x02\x12\x0c\n\x04open\x18\x02 \x01(\x02\x12\r\n\x05\x63lose\x18\x03 \x01(\x02\x12\x0c\n\x04high\x18\x04 \x01(\x02\x12\x0b\n\x03low\x18\x05 \x01(\x02\x12\x10\n\x08\x64\x61tetime\x18\x06 \x01(\t\"6\n\x06\x43\x61ndle\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x1c\n\x03\x62\x61r\x18\x02 \x01(\x0b\x32\x0f.playground.Bar\"M\n\x05Trade\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x13\n\x0b\x63reate_date\x18\x02 \x01(\t\x12\x10\n\x08quantity\x18\x03 \x01(\x01\x12\r\n\x05price\x18\x04 \x01(\x01\"\x9a\x02\n\x05Order\x12\n\n\x02id\x18\x01 \x01(\x04\x12\r\n\x05\x63lass\x18\x02 \x01(\t\x12\x0e\n\x06symbol\x18\x03 \x01(\t\x12\x0c\n\x04side\x18\x04 \x01(\t\x12\x10\n\x08quantity\x18\x05 \x01(\x02\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\x12\r\n\x05price\x18\x08 \x01(\x01\x12\x17\n\x0frequested_price\x18\t \x01(\x01\x12\x12\n\nstop_price\x18\n \x01(\x01\x12\x0b\n\x03tag\x18\x0b \x01(\t\x12!\n\x06trades\x18\x0c \x03(\x0b\x32\x11.playground.Trade\x12\x0e\n\x06status\x18\r \x01(\t\x12\x15\n\rreject_reason\x18\x0e \x01(\t\x12\x13\n\x0b\x63reate_date\x18\x0f \x01(\t2\x93\x03\n\x11PlaygroundService\x12\x64\n\x10\x43reatePlayground\x12*.playground.CreatePolygonPlaygroundRequest\x1a$.playground.CreatePlaygroundResponse\x12>\n\x08NextTick\x12\x1b.playground.NextTickRequest\x1a\x15.playground.TickDelta\x12>\n\nPlaceOrder\x12\x1d.playground.PlaceOrderRequest\x1a\x11.playground.Order\x12K\n\nGetAccount\x12\x1d.playground.GetAccountRequest\x1a\x1e.playground.GetAccountResponse\x12K\n\nGetCandles\x12\x1d.playground.GetCandlesRequest\x1a\x1e.playground.GetCandlesResponseB\x0eZ\x0c./playgroundb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'playground_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z\014./playground'
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._loaded_options = None
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_options = b'8\001'
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_start=33
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_end=189
  _globals['_GETCANDLESREQUEST']._serialized_start=191
  _globals['_GETCANDLESREQUEST']._serialized_end=289
  _globals['_GETCANDLESRESPONSE']._serialized_start=291
  _globals['_GETCANDLESRESPONSE']._serialized_end=342
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_start=344
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_end=382
  _globals['_GETACCOUNTREQUEST']._serialized_start=384
  _globals['_GETACCOUNTREQUEST']._serialized_end=448
  _globals['_GETACCOUNTRESPONSE']._serialized_start=451
  _globals['_GETACCOUNTRESPONSE']._serialized_end=698
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_start=628
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_end=698
  _globals['_POSITION']._serialized_start=700
  _globals['_POSITION']._serialized_end=788
  _globals['_PLACEORDERREQUEST']._serialized_start=791
  _globals['_PLACEORDERREQUEST']._serialized_end=934
  _globals['_NEXTTICKREQUEST']._serialized_start=936
  _globals['_NEXTTICKREQUEST']._serialized_end=1013
  _globals['_TICKDELTA']._serialized_start=1016
  _globals['_TICKDELTA']._serialized_end=1246
  _globals['_LIQUIDATIONEVENT']._serialized_start=1248
  _globals['_LIQUIDATIONEVENT']._serialized_end=1308
  _globals['_TICKDELTAEVENT']._serialized_start=1310
  _globals['_TICKDELTAEVENT']._serialized_end=1397
  _globals['_BAR']._serialized_start=1399
  _globals['_BAR']._serialized_end=1494
  _globals['_CANDLE']._serialized_start=1496
  _globals['_CANDLE']._serialized_end=1550
  _globals['_TRADE']._serialized_start=1552
  _globals['_TRADE']._serialized_end=1629
  _globals['_ORDER']._serialized_start=1632
  _globals['_ORDER']._serialized_end=1914
  _globals['_PLAYGROUNDSERVICE']._serialized_start=1917
  _globals['_PLAYGROUNDSERVICE']._serialized_end=2320
# @@protoc_insertion_point(module_scope)