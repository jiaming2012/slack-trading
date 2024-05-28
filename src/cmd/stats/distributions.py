from scipy.stats import (norm, lognorm, expon, t, beta, gamma, weibull_min, uniform,
                         chi2, logistic, laplace, pareto, cauchy, erlang)

distributions = {
    'Normal': norm,
    'Lognormal': lognorm,
    'Exponential': expon,
    't-Distribution': t,
    'Beta': beta,
    'Gamma': gamma,
    'Weibull': weibull_min,
    'Uniform': uniform,
    'Chi-Squared': chi2,
    'Logistic': logistic,
    'Laplace': laplace,
    'Pareto': pareto,
    'Cauchy': cauchy,
    'Erlang': erlang
}


def dist(data):
    ret = None
    if data['distribution'] == 'Normal':
        lower_limit = -float('inf')
        upper_limit = float('inf')
        ret = norm(*data['params'])
    elif data['distribution'] == 'Lognormal':
        lower_limit = 0
        upper_limit = float('inf')
        ret = lognorm(*data['params'])
    elif data['distribution'] == 'Exponential':
        lower_limit = 0
        upper_limit = float('inf')
        ret = expon(*data['params'])
    elif data['distribution'] == 't-Distribution':
        lower_limit = -float('inf')
        upper_limit = float('inf')
        ret = t(*data['params'])
    elif data['distribution'] == 'Beta':
        lower_limit = 0
        upper_limit = 1
        ret = beta(*data['params'])
    elif data['distribution'] == 'Gamma':
        lower_limit = 0
        upper_limit = float('inf')
        ret = gamma(*data['params'])
    elif data['distribution'] == 'Weibull':
        lower_limit = 0
        upper_limit = float('inf')
        ret = weibull_min(*data['params'])
    elif data['distribution'] == 'Uniform':
        lower_limit = data['params'][0]
        upper_limit = data['params'][1]
        ret =  uniform(*data['params'])
    elif data['distribution'] == 'Chi-Squared':
        lower_limit = 0
        upper_limit = float('inf')
        ret = chi2(*data['params'])
    elif data['distribution'] == 'Logistic':
        lower_limit = -float('inf')
        upper_limit = float('inf')
        ret = logistic(*data['params'])
    elif data['distribution'] == 'Laplace':
        lower_limit = -float('inf')
        upper_limit = float('inf')
        ret = laplace(*data['params'])
    elif data['distribution'] == 'Pareto':
        lower_limit = data['params'][0]
        upper_limit = float('inf')
        ret = pareto(*data['params'])
    elif data['distribution'] == 'Cauchy':
        lower_limit = -float('inf')
        upper_limit = float('inf')
        ret = cauchy(*data['params'])
    elif data['distribution'] == 'Erlang':
        lower_limit = 0
        upper_limit = float('inf')
        ret = erlang(*data['params'])
    else:
        raise ValueError(f"Invalid distribution: {data['distribution']}")
    
    return lower_limit, upper_limit, ret