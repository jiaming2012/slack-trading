from enum import Enum

class RepositorySource(Enum):
    CSV = 'csv'
    POLYGON = 'polygon'
    
class OrderSide(Enum):
    BUY = 'buy'
    SELL = 'sell'
    SELL_SHORT = 'sell_short'
    BUY_TO_COVER = 'buy_to_cover'
    BUY_TO_OPEN = 'buy_to_open'
    SELL_TO_CLOSE = 'sell_to_close'
    SELL_TO_OPEN = 'sell_to_open'
    BUY_TO_CLOSE = 'buy_to_close'
    
class LiveAccountType(Enum):
    MARGIN = 'margin'
    PAPER = 'paper'