# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import grpc
import warnings

import playground_pb2 as playground__pb2

GRPC_GENERATED_VERSION = '1.67.1'
GRPC_VERSION = grpc.__version__
_version_not_supported = False

try:
    from grpc._utilities import first_version_is_lower
    _version_not_supported = first_version_is_lower(GRPC_VERSION, GRPC_GENERATED_VERSION)
except ImportError:
    _version_not_supported = True

if _version_not_supported:
    raise RuntimeError(
        f'The grpc package installed is at version {GRPC_VERSION},'
        + f' but the generated code in playground_pb2_grpc.py depends on'
        + f' grpcio>={GRPC_GENERATED_VERSION}.'
        + f' Please upgrade your grpc module to grpcio>={GRPC_GENERATED_VERSION}'
        + f' or downgrade your generated code using grpcio-tools<={GRPC_VERSION}.'
    )


class PlaygroundServiceStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.CreatePlayground = channel.unary_unary(
                '/playground.PlaygroundService/CreatePlayground',
                request_serializer=playground__pb2.CreatePolygonPlaygroundRequest.SerializeToString,
                response_deserializer=playground__pb2.CreatePlaygroundResponse.FromString,
                _registered_method=True)
        self.NextTick = channel.unary_unary(
                '/playground.PlaygroundService/NextTick',
                request_serializer=playground__pb2.NextTickRequest.SerializeToString,
                response_deserializer=playground__pb2.TickDelta.FromString,
                _registered_method=True)
        self.PlaceOrder = channel.unary_unary(
                '/playground.PlaygroundService/PlaceOrder',
                request_serializer=playground__pb2.PlaceOrderRequest.SerializeToString,
                response_deserializer=playground__pb2.Order.FromString,
                _registered_method=True)
        self.GetAccount = channel.unary_unary(
                '/playground.PlaygroundService/GetAccount',
                request_serializer=playground__pb2.GetAccountRequest.SerializeToString,
                response_deserializer=playground__pb2.GetAccountResponse.FromString,
                _registered_method=True)
        self.GetCandles = channel.unary_unary(
                '/playground.PlaygroundService/GetCandles',
                request_serializer=playground__pb2.GetCandlesRequest.SerializeToString,
                response_deserializer=playground__pb2.GetCandlesResponse.FromString,
                _registered_method=True)


class PlaygroundServiceServicer(object):
    """Missing associated documentation comment in .proto file."""

    def CreatePlayground(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def NextTick(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def PlaceOrder(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetAccount(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetCandles(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')


def add_PlaygroundServiceServicer_to_server(servicer, server):
    rpc_method_handlers = {
            'CreatePlayground': grpc.unary_unary_rpc_method_handler(
                    servicer.CreatePlayground,
                    request_deserializer=playground__pb2.CreatePolygonPlaygroundRequest.FromString,
                    response_serializer=playground__pb2.CreatePlaygroundResponse.SerializeToString,
            ),
            'NextTick': grpc.unary_unary_rpc_method_handler(
                    servicer.NextTick,
                    request_deserializer=playground__pb2.NextTickRequest.FromString,
                    response_serializer=playground__pb2.TickDelta.SerializeToString,
            ),
            'PlaceOrder': grpc.unary_unary_rpc_method_handler(
                    servicer.PlaceOrder,
                    request_deserializer=playground__pb2.PlaceOrderRequest.FromString,
                    response_serializer=playground__pb2.Order.SerializeToString,
            ),
            'GetAccount': grpc.unary_unary_rpc_method_handler(
                    servicer.GetAccount,
                    request_deserializer=playground__pb2.GetAccountRequest.FromString,
                    response_serializer=playground__pb2.GetAccountResponse.SerializeToString,
            ),
            'GetCandles': grpc.unary_unary_rpc_method_handler(
                    servicer.GetCandles,
                    request_deserializer=playground__pb2.GetCandlesRequest.FromString,
                    response_serializer=playground__pb2.GetCandlesResponse.SerializeToString,
            ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
            'playground.PlaygroundService', rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))
    server.add_registered_method_handlers('playground.PlaygroundService', rpc_method_handlers)


 # This class is part of an EXPERIMENTAL API.
class PlaygroundService(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def CreatePlayground(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/playground.PlaygroundService/CreatePlayground',
            playground__pb2.CreatePolygonPlaygroundRequest.SerializeToString,
            playground__pb2.CreatePlaygroundResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def NextTick(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/playground.PlaygroundService/NextTick',
            playground__pb2.NextTickRequest.SerializeToString,
            playground__pb2.TickDelta.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def PlaceOrder(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/playground.PlaygroundService/PlaceOrder',
            playground__pb2.PlaceOrderRequest.SerializeToString,
            playground__pb2.Order.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def GetAccount(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/playground.PlaygroundService/GetAccount',
            playground__pb2.GetAccountRequest.SerializeToString,
            playground__pb2.GetAccountResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)

    @staticmethod
    def GetCandles(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(
            request,
            target,
            '/playground.PlaygroundService/GetCandles',
            playground__pb2.GetCandlesRequest.SerializeToString,
            playground__pb2.GetCandlesResponse.FromString,
            options,
            channel_credentials,
            insecure,
            call_credentials,
            compression,
            wait_for_ready,
            timeout,
            metadata,
            _registered_method=True)