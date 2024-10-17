import argparse
import os

parser = argparse.ArgumentParser()
parser.add_argument('--model', type=str, help='The name of the model to load', required=True)
parser.add_argument('--start', type=str, help='The start date of the backtest in YYYY-MM-DD format', required=True)
parser.add_argument('--end', type=str, help='The end date of the backtest in YYYY-MM-DD format', required=True)

args = parser.parse_args()

projectsDir = os.getenv('PROJECTS_DIR')
if projectsDir is None:
    raise ValueError('PROJECTS_DIR environment variable is not set')