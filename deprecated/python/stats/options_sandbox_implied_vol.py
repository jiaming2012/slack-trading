import numpy as np
from scipy.optimize import brentq

def binomial_option_price(S, K, T, r, sigma, N=100, option_type='call'):
    dt = T / N
    u = np.exp(sigma * np.sqrt(dt))
    d = 1 / u
    p = (np.exp(r * dt) - d) / (u - d)
    discount = np.exp(-r * dt)

    # Asset prices at maturity
    ST = np.array([S * u**j * d**(N - j) for j in range(N + 1)])

    # Option payoffs at maturity
    if option_type == 'call':
        option_values = np.maximum(ST - K, 0)
    else:
        option_values = np.maximum(K - ST, 0)

    # Backward induction
    for i in range(N - 1, -1, -1):
        option_values = discount * (p * option_values[1:] + (1 - p) * option_values[:-1])

    return option_values[0]

def implied_volatility_binomial(S, K, T, r, market_price, N=100, option_type='call'):
    def objective(sigma):
        price = binomial_option_price(S, K, T, r, sigma, N, option_type)
        return price - market_price

    # Use Brent's method to solve for the implied volatility
    return brentq(objective, 1e-5, 3.0)

# Example inputs
S = 100               # stock price
K = 100               # strike price
T = 1                 # time to maturity (1 year)
r = 0.05              # risk-free rate
market_price = 10.5   # observed option price
N = 100               # binomial steps

iv = implied_volatility_binomial(S, K, T, r, market_price, N, option_type='call')
print(f"Implied Volatility (Binomial): {iv:.2%}")
