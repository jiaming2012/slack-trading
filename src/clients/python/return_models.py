"""
Pluggable statistical models for forward-return distributions.

Provides an abstract interface and two implementations:
- EmpiricalModel: raw histogram (current behavior, baseline)
- BayesianNIGModel: Normal-InverseGamma conjugate for small-sample robustness

Standalone module — no server dependency.
"""

from __future__ import annotations

import math
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Dict, List, Optional

import numpy as np
from scipy import stats


# ------------------------------------------------------------------ #
# Result dataclass
# ------------------------------------------------------------------ #

@dataclass
class ReturnModelResult:
    """Summary output from a fitted return model."""

    mean: float
    stddev: float
    percentiles: Dict[str, float]  # {"5": ..., "10": ..., "25": ..., "50": ...}
    credible_interval_width: float
    sample_size: int
    model_name: str


# ------------------------------------------------------------------ #
# Abstract base
# ------------------------------------------------------------------ #

_DEFAULT_QUANTILES = [5, 10, 25, 50]


class ReturnModel(ABC):
    """Abstract interface for forward-return distribution models."""

    @abstractmethod
    def fit(self, returns: List[float], prior: Optional[dict] = None) -> None:
        """Fit the model to observed forward returns."""

    @abstractmethod
    def result(self) -> ReturnModelResult:
        """Return summary statistics from the fitted model."""

    @abstractmethod
    def percentile(self, q: float) -> float:
        """Return the q-th percentile (0–100) of the predictive distribution."""

    @abstractmethod
    def cdf(self, x: float) -> float:
        """P(return <= x) under the predictive distribution."""

    @abstractmethod
    def sample(self, n: int) -> List[float]:
        """Draw n samples from the predictive distribution."""


# ------------------------------------------------------------------ #
# EmpiricalModel
# ------------------------------------------------------------------ #

class EmpiricalModel(ReturnModel):
    """
    Wraps the current raw-histogram approach.

    Percentiles come directly from the observed data.
    CI width is the frequentist 2 × 1.96 × std / √n.
    """

    def __init__(self):
        self._returns: np.ndarray = np.array([])
        self._mean = 0.0
        self._std = 0.0

    def fit(self, returns: List[float], prior: Optional[dict] = None) -> None:
        if not returns:
            self._returns = np.array([])
            self._mean = 0.0
            self._std = 0.0
            return
        self._returns = np.array(returns, dtype=float)
        self._mean = float(np.mean(self._returns))
        self._std = float(np.std(self._returns, ddof=1)) if len(returns) > 1 else 0.0

    def result(self) -> ReturnModelResult:
        n = len(self._returns)
        if n == 0:
            return ReturnModelResult(
                mean=0.0, stddev=0.0,
                percentiles={str(q): 0.0 for q in _DEFAULT_QUANTILES},
                credible_interval_width=float("inf"),
                sample_size=0, model_name="empirical",
            )
        ci_width = 2 * 1.96 * self._std / math.sqrt(n) if n > 0 else float("inf")
        pcts = {
            str(q): float(np.percentile(self._returns, q))
            for q in _DEFAULT_QUANTILES
        }
        return ReturnModelResult(
            mean=self._mean, stddev=self._std,
            percentiles=pcts, credible_interval_width=ci_width,
            sample_size=n, model_name="empirical",
        )

    def percentile(self, q: float) -> float:
        if len(self._returns) == 0:
            return 0.0
        return float(np.percentile(self._returns, q))

    def cdf(self, x: float) -> float:
        if len(self._returns) == 0:
            return 0.5
        return float(np.mean(self._returns <= x))

    def sample(self, n: int) -> List[float]:
        if len(self._returns) == 0:
            return [0.0] * n
        return list(np.random.choice(self._returns, size=n, replace=True))


# ------------------------------------------------------------------ #
# BayesianNIGModel
# ------------------------------------------------------------------ #

class BayesianNIGModel(ReturnModel):
    """
    Normal-InverseGamma conjugate model.

    Prior on (mu, sigma^2):
        mu | sigma^2 ~ N(mu0, sigma^2 / kappa0)
        sigma^2 ~ InvGamma(alpha0, beta0)

    Posterior is also NIG with updated parameters.
    Predictive distribution is Student-t with 2*alpha_n degrees of freedom.

    Benefits over empirical:
    - Stable with small samples (shrinks toward prior)
    - Student-t predictive has heavier tails (appropriate for finance)
    - Credible intervals are Bayesian (narrower with prior info)
    """

    def __init__(
        self,
        mu0: float = 0.0,
        kappa0: float = 1.0,
        alpha0: float = 3.0,
        beta0: float = 0.001,
    ):
        # Prior hyperparameters
        self._mu0 = mu0
        self._kappa0 = kappa0
        self._alpha0 = alpha0
        self._beta0 = beta0

        # Posterior hyperparameters (set after fit)
        self._mu_n = mu0
        self._kappa_n = kappa0
        self._alpha_n = alpha0
        self._beta_n = beta0
        self._n = 0

    def fit(self, returns: List[float], prior: Optional[dict] = None) -> None:
        """
        Fit the model. Optionally override prior via dict with keys:
        mu0, kappa0, alpha0, beta0.
        """
        if prior:
            self._mu0 = prior.get("mu0", self._mu0)
            self._kappa0 = prior.get("kappa0", self._kappa0)
            self._alpha0 = prior.get("alpha0", self._alpha0)
            self._beta0 = prior.get("beta0", self._beta0)

        self._n = len(returns)
        if self._n == 0:
            self._mu_n = self._mu0
            self._kappa_n = self._kappa0
            self._alpha_n = self._alpha0
            self._beta_n = self._beta0
            return

        data = np.array(returns, dtype=float)
        x_bar = float(np.mean(data))
        n = self._n

        # Posterior update (conjugate)
        kappa_n = self._kappa0 + n
        mu_n = (self._kappa0 * self._mu0 + n * x_bar) / kappa_n
        alpha_n = self._alpha0 + n / 2.0

        ss = float(np.sum((data - x_bar) ** 2))  # sum of squared deviations
        beta_n = (
            self._beta0
            + 0.5 * ss
            + (self._kappa0 * n * (x_bar - self._mu0) ** 2) / (2.0 * kappa_n)
        )

        self._mu_n = mu_n
        self._kappa_n = kappa_n
        self._alpha_n = alpha_n
        self._beta_n = beta_n

    def _predictive_dist(self) -> stats.t:
        """
        Posterior predictive is Student-t:
            t_{2 alpha_n}(mu_n, beta_n * (kappa_n + 1) / (alpha_n * kappa_n))
        """
        df = 2 * self._alpha_n
        loc = self._mu_n
        scale = math.sqrt(
            self._beta_n * (self._kappa_n + 1) / (self._alpha_n * self._kappa_n)
        )
        return stats.t(df=df, loc=loc, scale=scale)

    def result(self) -> ReturnModelResult:
        dist = self._predictive_dist()
        mean = float(dist.mean())
        stddev = float(dist.std())

        pcts = {
            str(q): float(dist.ppf(q / 100.0))
            for q in _DEFAULT_QUANTILES
        }

        # 95% credible interval on the mean
        # Width of the marginal posterior on mu:
        # mu_n ± t_{alpha_n}(0.025) * sqrt(beta_n / (kappa_n * alpha_n))
        df_mean = 2 * self._alpha_n
        scale_mean = math.sqrt(self._beta_n / (self._kappa_n * self._alpha_n))
        ci_half = stats.t.ppf(0.975, df=df_mean) * scale_mean
        ci_width = 2 * ci_half

        return ReturnModelResult(
            mean=mean, stddev=stddev,
            percentiles=pcts, credible_interval_width=ci_width,
            sample_size=self._n, model_name="bayesian_nig",
        )

    def percentile(self, q: float) -> float:
        return float(self._predictive_dist().ppf(q / 100.0))

    def cdf(self, x: float) -> float:
        return float(self._predictive_dist().cdf(x))

    def sample(self, n: int) -> List[float]:
        return list(self._predictive_dist().rvs(size=n))


# ------------------------------------------------------------------ #
# Factory
# ------------------------------------------------------------------ #

def create_return_model(name: str = "bayesian_nig", **kwargs) -> ReturnModel:
    """
    Create a return model by name.

    Parameters
    ----------
    name : str
        "empirical" or "bayesian_nig".
    **kwargs
        Passed to the model constructor.
    """
    if name == "empirical":
        return EmpiricalModel()
    elif name == "bayesian_nig":
        return BayesianNIGModel(**kwargs)
    else:
        raise ValueError(f"Unknown return model: {name!r}")


def calibrate_prior(
    all_returns: List[float],
) -> dict:
    """
    Compute prior hyperparameters from pooled (unconditional) returns.

    Use this to calibrate the NIG prior from all signals' returns at a
    given horizon before fitting individual signal models.

    Parameters
    ----------
    all_returns : list[float]
        Pooled forward returns across all signals.

    Returns
    -------
    dict
        Prior params: mu0, kappa0, alpha0, beta0.
    """
    if len(all_returns) < 2:
        return {"mu0": 0.0, "kappa0": 1.0, "alpha0": 3.0, "beta0": 0.001}

    arr = np.array(all_returns, dtype=float)
    pooled_mean = float(np.mean(arr))
    pooled_var = float(np.var(arr, ddof=1))

    return {
        "mu0": pooled_mean,
        "kappa0": 1.0,    # 1 pseudo-observation (weak)
        "alpha0": 3.0,    # ensures finite prior variance
        "beta0": pooled_var * 2.0,  # centers prior on empirical variance
    }
