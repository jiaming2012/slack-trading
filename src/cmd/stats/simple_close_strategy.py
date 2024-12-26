from simple_base_strategy import SimpleBaseStrategy
import re

def parse_order_tag(tag):
    # Define the regular expression pattern
    pattern = r"sl__(?P<sl>\d+_\d+)__tp__(?P<tp>\d+_\d+)"
    
    # Match the pattern with the tag
    match = re.match(pattern, tag)
    
    if match:
        # Extract the sl and tp values
        sl = match.group('sl').replace('_', '.')
        tp = match.group('tp').replace('_', '.')
        return float(sl), float(tp)
    else:
        raise ValueError("Invalid tag format")
    
class SimpleCloseStrategy(SimpleBaseStrategy):
    pass
