from enum import Enum

class RepositorySource(Enum):
    CSV = 'csv'
    POLYGON = 'polygon'
    
class OrderSide(Enum):
    BUY = 'buy'
    SELL = 'sell'
    SELL_SHORT = 'sell_short'
    BUY_TO_COVER = 'buy_to_cover'
    
class LiveAccountType(Enum):
    MARGIN = 'margin'
    PAPER = 'paper'