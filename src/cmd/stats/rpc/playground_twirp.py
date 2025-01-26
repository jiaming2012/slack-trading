# -*- coding: utf-8 -*-
# Generated by https://github.com/verloop/twirpy/protoc-gen-twirpy.  DO NOT EDIT!
# source: playground.proto

from google.protobuf import symbol_database as _symbol_database

from twirp.base import Endpoint
from twirp.server import TwirpServer
from twirp.client import TwirpClient
try:
	from twirp.async_client import AsyncTwirpClient
	_async_available = True
except ModuleNotFoundError:
	_async_available = False

_sym_db = _symbol_database.Default()

class PlaygroundServiceServer(TwirpServer):

	def __init__(self, *args, service, server_path_prefix="/twirp"):
		super().__init__(service=service)
		self._prefix = F"{server_path_prefix}/playground.PlaygroundService"
		self._endpoints = {
			"CreatePlayground": Endpoint(
				service_name="PlaygroundService",
				name="CreatePlayground",
				function=getattr(service, "CreatePlayground"),
				input=_sym_db.GetSymbol("playground.CreatePolygonPlaygroundRequest"),
				output=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
			),
			"CreateLivePlayground": Endpoint(
				service_name="PlaygroundService",
				name="CreateLivePlayground",
				function=getattr(service, "CreateLivePlayground"),
				input=_sym_db.GetSymbol("playground.CreateLivePlaygroundRequest"),
				output=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
			),
			"GetPlaygrounds": Endpoint(
				service_name="PlaygroundService",
				name="GetPlaygrounds",
				function=getattr(service, "GetPlaygrounds"),
				input=_sym_db.GetSymbol("playground.GetPlaygroundsRequest"),
				output=_sym_db.GetSymbol("playground.GetPlaygroundsResponse"),
			),
			"NextTick": Endpoint(
				service_name="PlaygroundService",
				name="NextTick",
				function=getattr(service, "NextTick"),
				input=_sym_db.GetSymbol("playground.NextTickRequest"),
				output=_sym_db.GetSymbol("playground.TickDelta"),
			),
			"PlaceOrder": Endpoint(
				service_name="PlaygroundService",
				name="PlaceOrder",
				function=getattr(service, "PlaceOrder"),
				input=_sym_db.GetSymbol("playground.PlaceOrderRequest"),
				output=_sym_db.GetSymbol("playground.Order"),
			),
			"GetAccount": Endpoint(
				service_name="PlaygroundService",
				name="GetAccount",
				function=getattr(service, "GetAccount"),
				input=_sym_db.GetSymbol("playground.GetAccountRequest"),
				output=_sym_db.GetSymbol("playground.GetAccountResponse"),
			),
			"GetCandles": Endpoint(
				service_name="PlaygroundService",
				name="GetCandles",
				function=getattr(service, "GetCandles"),
				input=_sym_db.GetSymbol("playground.GetCandlesRequest"),
				output=_sym_db.GetSymbol("playground.GetCandlesResponse"),
			),
			"GetOpenOrders": Endpoint(
				service_name="PlaygroundService",
				name="GetOpenOrders",
				function=getattr(service, "GetOpenOrders"),
				input=_sym_db.GetSymbol("playground.GetOpenOrdersRequest"),
				output=_sym_db.GetSymbol("playground.GetOpenOrdersResponse"),
			),
			"SavePlayground": Endpoint(
				service_name="PlaygroundService",
				name="SavePlayground",
				function=getattr(service, "SavePlayground"),
				input=_sym_db.GetSymbol("playground.SavePlaygroundRequest"),
				output=_sym_db.GetSymbol("playground.EmptyResponse"),
			),
			"DeletePlayground": Endpoint(
				service_name="PlaygroundService",
				name="DeletePlayground",
				function=getattr(service, "DeletePlayground"),
				input=_sym_db.GetSymbol("playground.DeletePlaygroundRequest"),
				output=_sym_db.GetSymbol("playground.EmptyResponse"),
			),
		}

class PlaygroundServiceClient(TwirpClient):

	def CreatePlayground(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/CreatePlayground",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
			**kwargs,
		)

	def CreateLivePlayground(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/CreateLivePlayground",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
			**kwargs,
		)

	def GetPlaygrounds(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/GetPlaygrounds",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.GetPlaygroundsResponse"),
			**kwargs,
		)

	def NextTick(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/NextTick",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.TickDelta"),
			**kwargs,
		)

	def PlaceOrder(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/PlaceOrder",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.Order"),
			**kwargs,
		)

	def GetAccount(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/GetAccount",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.GetAccountResponse"),
			**kwargs,
		)

	def GetCandles(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/GetCandles",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.GetCandlesResponse"),
			**kwargs,
		)

	def GetOpenOrders(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/GetOpenOrders",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.GetOpenOrdersResponse"),
			**kwargs,
		)

	def SavePlayground(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/SavePlayground",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.EmptyResponse"),
			**kwargs,
		)

	def DeletePlayground(self, *args, ctx, request, server_path_prefix="/twirp", **kwargs):
		return self._make_request(
			url=F"{server_path_prefix}/playground.PlaygroundService/DeletePlayground",
			ctx=ctx,
			request=request,
			response_obj=_sym_db.GetSymbol("playground.EmptyResponse"),
			**kwargs,
		)


if _async_available:
	class AsyncPlaygroundServiceClient(AsyncTwirpClient):

		async def CreatePlayground(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/CreatePlayground",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
				session=session,
				**kwargs,
			)

		async def CreateLivePlayground(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/CreateLivePlayground",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.CreatePlaygroundResponse"),
				session=session,
				**kwargs,
			)

		async def GetPlaygrounds(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/GetPlaygrounds",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.GetPlaygroundsResponse"),
				session=session,
				**kwargs,
			)

		async def NextTick(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/NextTick",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.TickDelta"),
				session=session,
				**kwargs,
			)

		async def PlaceOrder(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/PlaceOrder",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.Order"),
				session=session,
				**kwargs,
			)

		async def GetAccount(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/GetAccount",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.GetAccountResponse"),
				session=session,
				**kwargs,
			)

		async def GetCandles(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/GetCandles",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.GetCandlesResponse"),
				session=session,
				**kwargs,
			)

		async def GetOpenOrders(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/GetOpenOrders",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.GetOpenOrdersResponse"),
				session=session,
				**kwargs,
			)

		async def SavePlayground(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/SavePlayground",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.EmptyResponse"),
				session=session,
				**kwargs,
			)

		async def DeletePlayground(self, *, ctx, request, server_path_prefix="/twirp", session=None, **kwargs):
			return await self._make_request(
				url=F"{server_path_prefix}/playground.PlaygroundService/DeletePlayground",
				ctx=ctx,
				request=request,
				response_obj=_sym_db.GetSymbol("playground.EmptyResponse"),
				session=session,
				**kwargs,
			)
