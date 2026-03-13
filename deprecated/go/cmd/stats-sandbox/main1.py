import numpy as np
import pymc as pm
import arviz as az
# import matplotlib.pyplot as plt
from sklearn.ensemble import RandomForestRegressor

random_state = 42

# Generate initial synthetic data
np.random.seed(random_state)
X = np.random.randn(100, 2)
y = 3 * X[:, 0] + 2 * X[:, 1] + np.random.randn(100) * 2

print('X:', X.shape)
print(X[:5])
print('y:', y.shape)
print(y[:5])

# Train a Random Forest Regressor
rf = RandomForestRegressor(n_estimators=100, random_state=random_state)
rf.fit(X, y)

# Make a prediction for a new data point
new_X = np.array([[4.5, 0.5], [0.25, 0.75]])
rf_pred = rf.predict(new_X)

print('Prediction:', rf_pred)
print('len: ', len(rf_pred))

# Get an estimate of the uncertainty (variance across trees)
tree_preds = np.array([tree.predict(new_X)[0] for tree in rf.estimators_])
rf_var = np.var(tree_preds)
pred2 = np.mean(tree_preds)

print('Uncertainty:', rf_var)

# Function to update the Bayesian model with accumulated data
def update_bayesian_model(predictions, variances):
    with pm.Model() as model:
        # Prior distribution
        parameter = pm.Gamma('parameter', alpha=2, beta=1)
        
        # Likelihood function for each prediction
        for i, (pred, var) in enumerate(zip(predictions, variances)):
            likelihood = pm.StudentT(f'likelihood_{i}', nu=5, mu=pred, sigma=np.sqrt(var), observed=pred)
        
        # Sampling from the posterior with a single core
        trace = pm.sample(2000, return_inferencedata=True, progressbar=True, cores=1, chains=4, target_accept=0.95)
        
    return trace

# Initialize lists to accumulate predictions and variances
predictions = [rf_pred]
variances = [rf_var]

# Initial Bayesian model update
trace = update_bayesian_model(predictions, variances)

# Plotting the initial posterior distribution
az.plot_posterior(trace, var_names=['parameter'], round_to=2)
plt.show()

# Iteratively update the Bayesian model with new data
for i in range(5):  # 5 iterations
    # Generate new synthetic data
    new_X = np.random.randn(100, 2)
    new_y = 3 * new_X[:, 0] + 2 * new_X[:, 1] + np.random.randn(100) * 2
    
    # Update the Random Forest Regressor with new data
    rf.fit(new_X, new_y)
    
    # Make a new prediction
    new_rf_pred = rf.predict(np.array([[0.5, 0.5]]))
    
    # Get an estimate of the new uncertainty
    new_tree_preds = np.array([tree.predict(np.array([[0.5, 0.5]]))[0] for tree in rf.estimators_])
    new_rf_var = np.var(new_tree_preds)
    
    print(f'Iteration {i+1}: New Prediction: {new_rf_pred}, New Uncertainty: {new_rf_var}')
    
    # Accumulate the new prediction and variance
    predictions.append(new_rf_pred)
    variances.append(new_rf_var)
    
    # Update the Bayesian model with accumulated data
    trace = update_bayesian_model(predictions, variances)
    
    # Plotting the updated posterior distribution
    az.plot_posterior(trace, var_names=['parameter'], round_to=2)
    # plt.show()