"""Tests for return_models module."""

import math

import numpy as np
import pytest

from lib.return_models import (
    BayesianNIGModel,
    EmpiricalModel,
    ReturnModelResult,
    calibrate_prior,
    create_return_model,
)


# ------------------------------------------------------------------ #
# Helpers
# ------------------------------------------------------------------ #

def _make_returns(n=200, mean=0.005, std=0.02, seed=42):
    rng = np.random.default_rng(seed)
    return list(rng.normal(mean, std, n))


# ------------------------------------------------------------------ #
# TestEmpiricalModel
# ------------------------------------------------------------------ #

class TestEmpiricalModel:

    def test_fit_and_percentiles(self):
        """Known returns → percentiles match numpy."""
        returns = _make_returns(200)
        m = EmpiricalModel()
        m.fit(returns)
        result = m.result()
        assert result.model_name == "empirical"
        assert result.sample_size == 200
        # 50th percentile ≈ median
        expected_median = float(np.percentile(returns, 50))
        assert abs(result.percentiles["50"] - expected_median) < 1e-10

    def test_ci_width_matches_formula(self):
        """CI width = 2 × 1.96 × std / √n."""
        returns = _make_returns(100)
        m = EmpiricalModel()
        m.fit(returns)
        result = m.result()
        expected = 2 * 1.96 * result.stddev / math.sqrt(100)
        assert abs(result.credible_interval_width - expected) < 1e-10

    def test_cdf_at_zero(self):
        """CDF(0) = fraction of returns <= 0."""
        returns = [-0.03, -0.01, 0.0, 0.01, 0.02]
        m = EmpiricalModel()
        m.fit(returns)
        # 3 out of 5 are <= 0
        assert abs(m.cdf(0.0) - 0.6) < 1e-10

    def test_percentile_method(self):
        """percentile() matches numpy."""
        returns = _make_returns(100)
        m = EmpiricalModel()
        m.fit(returns)
        assert abs(m.percentile(25) - float(np.percentile(returns, 25))) < 1e-10

    def test_empty_returns(self):
        """fit([]) → safe defaults, no crash."""
        m = EmpiricalModel()
        m.fit([])
        result = m.result()
        assert result.sample_size == 0
        assert result.mean == 0.0
        assert result.credible_interval_width == float("inf")

    def test_single_return(self):
        """Single observation → stddev=0, finite result."""
        m = EmpiricalModel()
        m.fit([0.01])
        result = m.result()
        assert result.sample_size == 1
        assert result.stddev == 0.0
        assert result.mean == 0.01


# ------------------------------------------------------------------ #
# TestBayesianNIGModel
# ------------------------------------------------------------------ #

class TestBayesianNIGModel:

    def test_posterior_shrinks_to_prior_with_small_n(self):
        """n=3 → posterior mean close to prior mu0, not sample mean."""
        returns = [0.10, 0.12, 0.08]  # sample mean = 0.10
        m = BayesianNIGModel(mu0=0.0, kappa0=1.0, alpha0=3.0, beta0=0.001)
        m.fit(returns)
        result = m.result()
        # With kappa0=1 and n=3, posterior mean = (1*0 + 3*0.1) / 4 = 0.075
        # Should be between prior (0) and sample mean (0.1)
        assert 0.0 < result.mean < 0.10
        assert abs(result.mean - 0.075) < 0.01

    def test_posterior_approaches_empirical_with_large_n(self):
        """n=500 → posterior mean ≈ sample mean."""
        returns = _make_returns(500, mean=0.02, std=0.01)
        sample_mean = float(np.mean(returns))
        m = BayesianNIGModel(mu0=0.0, kappa0=1.0, alpha0=3.0, beta0=0.001)
        m.fit(returns)
        result = m.result()
        # With n=500, kappa0=1, posterior mean ≈ sample mean
        assert abs(result.mean - sample_mean) < 0.001

    def test_credible_interval_narrower_than_frequentist(self):
        """For small n, Bayesian CI should be narrower (prior regularizes)."""
        returns = _make_returns(10, mean=0.005, std=0.02)
        # Empirical CI
        emp = EmpiricalModel()
        emp.fit(returns)
        emp_ci = emp.result().credible_interval_width
        # Bayesian CI with informative prior
        prior = calibrate_prior(_make_returns(1000, mean=0.003, std=0.015))
        bay = BayesianNIGModel(**prior)
        bay.fit(returns)
        bay_ci = bay.result().credible_interval_width
        # Bayesian should be narrower (prior provides information)
        assert bay_ci < emp_ci

    def test_heavy_tails_student_t(self):
        """Predictive 5th percentile further from mean than Normal.

        Use very small sample (n=5) so degrees of freedom are low,
        making the heavy-tail effect pronounced.
        """
        returns = [0.01, -0.02, 0.005, -0.015, 0.0]
        m = BayesianNIGModel(mu0=0.0, kappa0=1.0, alpha0=1.5, beta0=0.0001)
        m.fit(returns)
        result = m.result()
        # With low df, Student-t 1st percentile is much more extreme than Normal
        from scipy.stats import norm
        normal_1st = norm.ppf(0.01, loc=result.mean, scale=result.stddev)
        student_1st = m.percentile(1)
        assert student_1st < normal_1st

    def test_prior_calibration(self):
        """calibrate_prior sets mu0 and beta0 from pooled stats."""
        pooled = _make_returns(500, mean=0.003, std=0.015)
        prior = calibrate_prior(pooled)
        assert abs(prior["mu0"] - float(np.mean(pooled))) < 1e-6
        assert prior["kappa0"] == 1.0
        assert prior["alpha0"] == 3.0
        expected_beta = float(np.var(pooled, ddof=1)) * 2.0
        assert abs(prior["beta0"] - expected_beta) < 1e-6

    def test_single_observation(self):
        """n=1 → valid posterior dominated by prior, no crash."""
        m = BayesianNIGModel(mu0=0.0, kappa0=1.0, alpha0=3.0, beta0=0.001)
        m.fit([0.05])
        result = m.result()
        assert result.sample_size == 1
        # Posterior mean = (1*0 + 1*0.05) / 2 = 0.025
        assert abs(result.mean - 0.025) < 0.01
        assert result.stddev > 0

    def test_all_same_returns(self):
        """All identical returns → posterior stddev > 0 (prior prevents zero)."""
        m = BayesianNIGModel(mu0=0.0, kappa0=1.0, alpha0=3.0, beta0=0.001)
        m.fit([0.01] * 20)
        result = m.result()
        assert result.stddev > 0

    def test_cdf_monotonic(self):
        """CDF is monotonically increasing."""
        m = BayesianNIGModel()
        m.fit(_make_returns(100))
        xs = [-0.05, -0.02, 0.0, 0.02, 0.05]
        cdfs = [m.cdf(x) for x in xs]
        for i in range(1, len(cdfs)):
            assert cdfs[i] >= cdfs[i - 1]

    def test_sample_returns_list(self):
        """sample() returns the requested number of samples."""
        m = BayesianNIGModel()
        m.fit(_make_returns(50))
        samples = m.sample(100)
        assert len(samples) == 100
        assert all(isinstance(s, float) for s in samples)

    def test_fit_with_explicit_prior(self):
        """Prior can be overridden via dict."""
        m = BayesianNIGModel(mu0=0.0)
        m.fit([0.01, 0.02, 0.03], prior={"mu0": 0.05, "kappa0": 5.0})
        result = m.result()
        # Strong prior at 0.05 with kappa0=5 should pull mean toward 0.05
        assert result.mean > 0.02  # sample mean
        assert result.mean < 0.05  # prior mean

    def test_empty_returns(self):
        """fit([]) → prior is the posterior."""
        m = BayesianNIGModel(mu0=0.01, kappa0=1.0, alpha0=3.0, beta0=0.001)
        m.fit([])
        result = m.result()
        assert result.sample_size == 0
        assert abs(result.mean - 0.01) < 1e-6


# ------------------------------------------------------------------ #
# TestModelFactory
# ------------------------------------------------------------------ #

class TestModelFactory:

    def test_create_empirical(self):
        m = create_return_model("empirical")
        assert isinstance(m, EmpiricalModel)

    def test_create_bayesian_nig(self):
        m = create_return_model("bayesian_nig")
        assert isinstance(m, BayesianNIGModel)

    def test_create_with_kwargs(self):
        m = create_return_model("bayesian_nig", mu0=0.05)
        assert isinstance(m, BayesianNIGModel)

    def test_unknown_model_raises(self):
        with pytest.raises(ValueError, match="Unknown return model"):
            create_return_model("random_forest")


# ------------------------------------------------------------------ #
# TestCalibratePrior
# ------------------------------------------------------------------ #

class TestCalibratePrior:

    def test_empty_returns(self):
        prior = calibrate_prior([])
        assert prior["mu0"] == 0.0
        assert prior["kappa0"] == 1.0

    def test_single_return(self):
        prior = calibrate_prior([0.01])
        assert prior["mu0"] == 0.0  # falls back to defaults

    def test_normal_returns(self):
        returns = _make_returns(1000, mean=0.003, std=0.015)
        prior = calibrate_prior(returns)
        assert abs(prior["mu0"] - 0.003) < 0.002
        assert prior["beta0"] > 0
