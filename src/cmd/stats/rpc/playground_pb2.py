# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# NO CHECKED-IN PROTOBUF GENCODE
# source: playground.proto
# Protobuf Python Version: 5.28.2
"""Generated protocol buffer code."""
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import runtime_version as _runtime_version
from google.protobuf import symbol_database as _symbol_database
from google.protobuf.internal import builder as _builder
_runtime_version.ValidateProtobufRuntimeVersion(
    _runtime_version.Domain.PUBLIC,
    5,
    28,
    2,
    '',
    'playground.proto'
)
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from google.protobuf import timestamp_pb2 as google_dot_protobuf_dot_timestamp__pb2
from google.protobuf import duration_pb2 as google_dot_protobuf_dot_duration__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x10playground.proto\x12\nplayground\x1a\x1fgoogle/protobuf/timestamp.proto\x1a\x1egoogle/protobuf/duration.proto\"\x17\n\x15GetPlaygroundsRequest\"4\n\x04Meta\x12\x17\n\x0finitial_balance\x18\x01 \x01(\x01\x12\x13\n\x0b\x65nvironment\x18\x02 \x01(\t\"\xd0\x01\n\x11PlaygroundSession\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x1e\n\x04meta\x18\x02 \x01(\x0b\x32\x10.playground.Meta\x12 \n\x05\x63lock\x18\x03 \x01(\x0b\x32\x11.playground.Clock\x12,\n\x0crepositories\x18\x04 \x03(\x0b\x32\x16.playground.Repository\x12\x0f\n\x07\x62\x61lance\x18\x05 \x01(\x01\x12\x0e\n\x06\x65quity\x18\x06 \x01(\x01\x12\x13\n\x0b\x66ree_margin\x18\x07 \x01(\x01\"L\n\x16GetPlaygroundsResponse\x12\x32\n\x0bplaygrounds\x18\x01 \x03(\x0b\x32\x1d.playground.PlaygroundSession\":\n\x15GetOpenOrdersResponse\x12!\n\x06orders\x18\x01 \x03(\x0b\x32\x11.playground.Order\"=\n\x14GetOpenOrdersRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\".\n\x15SavePlaygroundRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\"\x0f\n\rEmptyResponse\"H\n\x05\x43lock\x12\x14\n\x0c\x63urrent_time\x18\x01 \x01(\t\x12\r\n\x05start\x18\x02 \x01(\t\x12\x11\n\x04stop\x18\x03 \x01(\tH\x00\x88\x01\x01\x42\x07\n\x05_stop\"}\n\nRepository\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x1b\n\x13timespan_multiplier\x18\x02 \x01(\r\x12\x15\n\rtimespan_unit\x18\x03 \x01(\t\x12\x12\n\nindicators\x18\x04 \x03(\t\x12\x17\n\x0fhistory_in_days\x18\x05 \x01(\r\"\x9b\x01\n\x1e\x43reatePolygonPlaygroundRequest\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x01\x12\x12\n\nstart_date\x18\x02 \x01(\t\x12\x11\n\tstop_date\x18\x03 \x01(\t\x12,\n\x0crepositories\x18\x04 \x03(\x0b\x32\x16.playground.Repository\x12\x13\n\x0b\x65nvironment\x18\x05 \x01(\t\"0\n\x17\x44\x65letePlaygroundRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\"\xc0\x01\n\x1b\x43reateLivePlaygroundRequest\x12\x0f\n\x07\x62\x61lance\x18\x01 \x01(\x01\x12\x15\n\rsource_broker\x18\x02 \x01(\t\x12\x19\n\x11source_account_id\x18\x03 \x01(\t\x12\x1b\n\x13source_api_key_name\x18\x04 \x01(\t\x12,\n\x0crepositories\x18\x05 \x03(\x0b\x32\x16.playground.Repository\x12\x13\n\x0b\x65nvironment\x18\x06 \x01(\t\"}\n\x11GetCandlesRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x19\n\x11period_in_seconds\x18\x03 \x01(\x05\x12\x13\n\x0b\x66romRTF3339\x18\x04 \x01(\t\x12\x11\n\ttoRTF3339\x18\x05 \x01(\t\"3\n\x12GetCandlesResponse\x12\x1d\n\x04\x62\x61rs\x18\x01 \x03(\x0b\x32\x0f.playground.Bar\"&\n\x18\x43reatePlaygroundResponse\x12\n\n\x02id\x18\x01 \x01(\t\"\x85\x01\n\x0b\x41\x63\x63ountMeta\x12\x12\n\nstart_date\x18\x01 \x01(\t\x12\x15\n\x08\x65nd_date\x18\x02 \x01(\tH\x00\x88\x01\x01\x12\x0f\n\x07symbols\x18\x03 \x03(\t\x12\x18\n\x10starting_balance\x18\x04 \x01(\x01\x12\x13\n\x0b\x65nvironment\x18\x05 \x01(\tB\x0b\n\t_end_date\"@\n\x11GetAccountRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x14\n\x0c\x66\x65tch_orders\x18\x02 \x01(\x08\"\x9e\x02\n\x12GetAccountResponse\x12%\n\x04meta\x18\x01 \x01(\x0b\x32\x17.playground.AccountMeta\x12\x0f\n\x07\x62\x61lance\x18\x02 \x01(\x01\x12\x0e\n\x06\x65quity\x18\x03 \x01(\x01\x12\x13\n\x0b\x66ree_margin\x18\x04 \x01(\x01\x12!\n\x06orders\x18\x05 \x03(\x0b\x32\x11.playground.Order\x12@\n\tpositions\x18\x06 \x03(\x0b\x32-.playground.GetAccountResponse.PositionsEntry\x1a\x46\n\x0ePositionsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12#\n\x05value\x18\x02 \x01(\x0b\x32\x14.playground.Position:\x02\x38\x01\"X\n\x08Position\x12\x10\n\x08quantity\x18\x01 \x01(\x01\x12\x12\n\ncost_basis\x18\x02 \x01(\x01\x12\n\n\x02pl\x18\x03 \x01(\x01\x12\x1a\n\x12maintenance_margin\x18\x04 \x01(\x01\"\xb5\x01\n\x11PlaceOrderRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0e\n\x06symbol\x18\x02 \x01(\t\x12\x13\n\x0b\x61sset_class\x18\x03 \x01(\t\x12\x10\n\x08quantity\x18\x04 \x01(\x01\x12\x0c\n\x04side\x18\x05 \x01(\t\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\x12\x0b\n\x03tag\x18\x08 \x01(\t\x12\x17\n\x0frequested_price\x18\t \x01(\x01\"M\n\x0fNextTickRequest\x12\x15\n\rplayground_id\x18\x01 \x01(\t\x12\x0f\n\x07seconds\x18\x02 \x01(\x04\x12\x12\n\nis_preview\x18\x03 \x01(\x08\"\xe6\x01\n\tTickDelta\x12%\n\nnew_trades\x18\x01 \x03(\x0b\x32\x11.playground.Trade\x12\'\n\x0bnew_candles\x18\x02 \x03(\x0b\x32\x12.playground.Candle\x12)\n\x0einvalid_orders\x18\x03 \x03(\x0b\x32\x11.playground.Order\x12*\n\x06\x65vents\x18\x04 \x03(\x0b\x32\x1a.playground.TickDeltaEvent\x12\x14\n\x0c\x63urrent_time\x18\x05 \x01(\t\x12\x1c\n\x14is_backtest_complete\x18\x06 \x01(\x08\"<\n\x10LiquidationEvent\x12(\n\rorders_placed\x18\x01 \x03(\x0b\x32\x11.playground.Order\"W\n\x0eTickDeltaEvent\x12\x0c\n\x04type\x18\x01 \x01(\t\x12\x37\n\x11liquidation_event\x18\x02 \x01(\x0b\x32\x1c.playground.LiquidationEvent\"\xa2\x06\n\x03\x42\x61r\x12\x0e\n\x06volume\x18\x01 \x01(\x01\x12\x0c\n\x04open\x18\x02 \x01(\x01\x12\r\n\x05\x63lose\x18\x03 \x01(\x01\x12\x0c\n\x04high\x18\x04 \x01(\x01\x12\x0b\n\x03low\x18\x05 \x01(\x01\x12\x10\n\x08\x64\x61tetime\x18\x06 \x01(\t\x12\x13\n\x0bsuperT_50_3\x18\x07 \x01(\x01\x12\x13\n\x0bsuperD_50_3\x18\x08 \x01(\x05\x12\x13\n\x0bsuperL_50_3\x18\t \x01(\x01\x12\x13\n\x0bsuperS_50_3\x18\n \x01(\x01\x12\x1c\n\x14stochrsi_k_14_14_3_3\x18\x0b \x01(\x01\x12\x1c\n\x14stochrsi_d_14_14_3_3\x18\x0c \x01(\x01\x12\x0e\n\x06\x61tr_14\x18\r \x01(\x01\x12\x0e\n\x06sma_50\x18\x0e \x01(\x01\x12\x0f\n\x07sma_100\x18\x0f \x01(\x01\x12\x0f\n\x07sma_200\x18\x10 \x01(\x01\x12\x1f\n\x17stochrsi_cross_above_20\x18\x11 \x01(\x08\x12\x1f\n\x17stochrsi_cross_below_80\x18\x12 \x01(\x08\x12\x13\n\x0b\x63lose_lag_1\x18\x13 \x01(\x01\x12\x13\n\x0b\x63lose_lag_2\x18\x14 \x01(\x01\x12\x13\n\x0b\x63lose_lag_3\x18\x15 \x01(\x01\x12\x13\n\x0b\x63lose_lag_4\x18\x16 \x01(\x01\x12\x13\n\x0b\x63lose_lag_5\x18\x17 \x01(\x01\x12\x13\n\x0b\x63lose_lag_6\x18\x18 \x01(\x01\x12\x13\n\x0b\x63lose_lag_7\x18\x19 \x01(\x01\x12\x13\n\x0b\x63lose_lag_8\x18\x1a \x01(\x01\x12\x13\n\x0b\x63lose_lag_9\x18\x1b \x01(\x01\x12\x14\n\x0c\x63lose_lag_10\x18\x1c \x01(\x01\x12\x14\n\x0c\x63lose_lag_11\x18\x1d \x01(\x01\x12\x14\n\x0c\x63lose_lag_12\x18\x1e \x01(\x01\x12\x14\n\x0c\x63lose_lag_13\x18\x1f \x01(\x01\x12\x14\n\x0c\x63lose_lag_14\x18  \x01(\x01\x12\x14\n\x0c\x63lose_lag_15\x18! \x01(\x01\x12\x14\n\x0c\x63lose_lag_16\x18\" \x01(\x01\x12\x14\n\x0c\x63lose_lag_17\x18# \x01(\x01\x12\x14\n\x0c\x63lose_lag_18\x18$ \x01(\x01\x12\x14\n\x0c\x63lose_lag_19\x18% \x01(\x01\x12\x14\n\x0c\x63lose_lag_20\x18& \x01(\x01\"F\n\x06\x43\x61ndle\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x0e\n\x06period\x18\x02 \x01(\x05\x12\x1c\n\x03\x62\x61r\x18\x03 \x01(\x0b\x32\x0f.playground.Bar\"M\n\x05Trade\x12\x0e\n\x06symbol\x18\x01 \x01(\t\x12\x13\n\x0b\x63reate_date\x18\x02 \x01(\t\x12\x10\n\x08quantity\x18\x03 \x01(\x01\x12\r\n\x05price\x18\x04 \x01(\x01\"\xe3\x02\n\x05Order\x12\n\n\x02id\x18\x01 \x01(\x04\x12\r\n\x05\x63lass\x18\x02 \x01(\t\x12\x0e\n\x06symbol\x18\x03 \x01(\t\x12\x0c\n\x04side\x18\x04 \x01(\t\x12\x10\n\x08quantity\x18\x05 \x01(\x01\x12\x0c\n\x04type\x18\x06 \x01(\t\x12\x10\n\x08\x64uration\x18\x07 \x01(\t\x12\r\n\x05price\x18\x08 \x01(\x01\x12\x17\n\x0frequested_price\x18\t \x01(\x01\x12\x12\n\nstop_price\x18\n \x01(\x01\x12\x0b\n\x03tag\x18\x0b \x01(\t\x12!\n\x06trades\x18\x0c \x03(\x0b\x32\x11.playground.Trade\x12\x0e\n\x06status\x18\r \x01(\t\x12\x15\n\rreject_reason\x18\x0e \x01(\t\x12\x13\n\x0b\x63reate_date\x18\x0f \x01(\t\x12$\n\tclosed_by\x18\x10 \x03(\x0b\x32\x11.playground.Trade\x12!\n\x06\x63loses\x18\x11 \x03(\x0b\x32\x11.playground.Order2\xcd\x06\n\x11PlaygroundService\x12\x64\n\x10\x43reatePlayground\x12*.playground.CreatePolygonPlaygroundRequest\x1a$.playground.CreatePlaygroundResponse\x12\x65\n\x14\x43reateLivePlayground\x12\'.playground.CreateLivePlaygroundRequest\x1a$.playground.CreatePlaygroundResponse\x12W\n\x0eGetPlaygrounds\x12!.playground.GetPlaygroundsRequest\x1a\".playground.GetPlaygroundsResponse\x12>\n\x08NextTick\x12\x1b.playground.NextTickRequest\x1a\x15.playground.TickDelta\x12>\n\nPlaceOrder\x12\x1d.playground.PlaceOrderRequest\x1a\x11.playground.Order\x12K\n\nGetAccount\x12\x1d.playground.GetAccountRequest\x1a\x1e.playground.GetAccountResponse\x12K\n\nGetCandles\x12\x1d.playground.GetCandlesRequest\x1a\x1e.playground.GetCandlesResponse\x12T\n\rGetOpenOrders\x12 .playground.GetOpenOrdersRequest\x1a!.playground.GetOpenOrdersResponse\x12N\n\x0eSavePlayground\x12!.playground.SavePlaygroundRequest\x1a\x19.playground.EmptyResponse\x12R\n\x10\x44\x65letePlayground\x12#.playground.DeletePlaygroundRequest\x1a\x19.playground.EmptyResponseB\x0eZ\x0c./playgroundb\x06proto3')

_globals = globals()
_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, _globals)
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'playground_pb2', _globals)
if not _descriptor._USE_C_DESCRIPTORS:
  _globals['DESCRIPTOR']._loaded_options = None
  _globals['DESCRIPTOR']._serialized_options = b'Z\014./playground'
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._loaded_options = None
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_options = b'8\001'
  _globals['_GETPLAYGROUNDSREQUEST']._serialized_start=97
  _globals['_GETPLAYGROUNDSREQUEST']._serialized_end=120
  _globals['_META']._serialized_start=122
  _globals['_META']._serialized_end=174
  _globals['_PLAYGROUNDSESSION']._serialized_start=177
  _globals['_PLAYGROUNDSESSION']._serialized_end=385
  _globals['_GETPLAYGROUNDSRESPONSE']._serialized_start=387
  _globals['_GETPLAYGROUNDSRESPONSE']._serialized_end=463
  _globals['_GETOPENORDERSRESPONSE']._serialized_start=465
  _globals['_GETOPENORDERSRESPONSE']._serialized_end=523
  _globals['_GETOPENORDERSREQUEST']._serialized_start=525
  _globals['_GETOPENORDERSREQUEST']._serialized_end=586
  _globals['_SAVEPLAYGROUNDREQUEST']._serialized_start=588
  _globals['_SAVEPLAYGROUNDREQUEST']._serialized_end=634
  _globals['_EMPTYRESPONSE']._serialized_start=636
  _globals['_EMPTYRESPONSE']._serialized_end=651
  _globals['_CLOCK']._serialized_start=653
  _globals['_CLOCK']._serialized_end=725
  _globals['_REPOSITORY']._serialized_start=727
  _globals['_REPOSITORY']._serialized_end=852
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_start=855
  _globals['_CREATEPOLYGONPLAYGROUNDREQUEST']._serialized_end=1010
  _globals['_DELETEPLAYGROUNDREQUEST']._serialized_start=1012
  _globals['_DELETEPLAYGROUNDREQUEST']._serialized_end=1060
  _globals['_CREATELIVEPLAYGROUNDREQUEST']._serialized_start=1063
  _globals['_CREATELIVEPLAYGROUNDREQUEST']._serialized_end=1255
  _globals['_GETCANDLESREQUEST']._serialized_start=1257
  _globals['_GETCANDLESREQUEST']._serialized_end=1382
  _globals['_GETCANDLESRESPONSE']._serialized_start=1384
  _globals['_GETCANDLESRESPONSE']._serialized_end=1435
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_start=1437
  _globals['_CREATEPLAYGROUNDRESPONSE']._serialized_end=1475
  _globals['_ACCOUNTMETA']._serialized_start=1478
  _globals['_ACCOUNTMETA']._serialized_end=1611
  _globals['_GETACCOUNTREQUEST']._serialized_start=1613
  _globals['_GETACCOUNTREQUEST']._serialized_end=1677
  _globals['_GETACCOUNTRESPONSE']._serialized_start=1680
  _globals['_GETACCOUNTRESPONSE']._serialized_end=1966
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_start=1896
  _globals['_GETACCOUNTRESPONSE_POSITIONSENTRY']._serialized_end=1966
  _globals['_POSITION']._serialized_start=1968
  _globals['_POSITION']._serialized_end=2056
  _globals['_PLACEORDERREQUEST']._serialized_start=2059
  _globals['_PLACEORDERREQUEST']._serialized_end=2240
  _globals['_NEXTTICKREQUEST']._serialized_start=2242
  _globals['_NEXTTICKREQUEST']._serialized_end=2319
  _globals['_TICKDELTA']._serialized_start=2322
  _globals['_TICKDELTA']._serialized_end=2552
  _globals['_LIQUIDATIONEVENT']._serialized_start=2554
  _globals['_LIQUIDATIONEVENT']._serialized_end=2614
  _globals['_TICKDELTAEVENT']._serialized_start=2616
  _globals['_TICKDELTAEVENT']._serialized_end=2703
  _globals['_BAR']._serialized_start=2706
  _globals['_BAR']._serialized_end=3508
  _globals['_CANDLE']._serialized_start=3510
  _globals['_CANDLE']._serialized_end=3580
  _globals['_TRADE']._serialized_start=3582
  _globals['_TRADE']._serialized_end=3659
  _globals['_ORDER']._serialized_start=3662
  _globals['_ORDER']._serialized_end=4017
  _globals['_PLAYGROUNDSERVICE']._serialized_start=4020
  _globals['_PLAYGROUNDSERVICE']._serialized_end=4865
# @@protoc_insertion_point(module_scope)
