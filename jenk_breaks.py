import random
import jenkspy
import pandas as pd
import numpy as np
from sklearn.cluster import DBSCAN
import hdbscan


def generate_weighted_samples(n=1000, buckets=None):
    """
    Generate `n` random integers in [0..100] such that most values fall into
    the user-specified buckets.
    
    Parameters:
    - n: total number of samples
    - buckets: list of tuples (low, high, weight), where:
        low, high: inclusive ends of the bucket range
        weight: desired fraction of total samples in this bucket
    
    Returns:
    - samples: list of n integers in [0..100]
    """
    if buckets is None:
        buckets = []
    
    # Compute total bucket weight, and leftover for “other”
    total_w = sum(w for _, _, w in buckets)
    other_w = max(0.0, 1.0 - total_w)
    
    # Build choices: each bucket as a range, plus None=“other”
    choices = [(range(low, high+1), w) for (low, high, w) in buckets]
    choices.append((None, other_w))
    
    ranges, weights = zip(*choices)
    
    # Precompute “other” pool once
    excluded = set()
    for low, high, _ in buckets:
        excluded.update(range(low, high+1))
    other_pool = [i for i in range(101) if i not in excluded]
    
    samples = []
    for _ in range(n):
        sel = random.choices(ranges, weights)[0]
        if sel is None:
            samples.append(random.choice(other_pool))
        else:
            samples.append(random.choice(list(sel)))
    return samples


def find_optimal_k(data, max_k=10, penalty=1.0):
    best = {'k': None, 'cost': np.inf, 'breaks': None}
    n, total_var = len(data), np.var(data, ddof=0)
    
    for k in range(2, max_k+1):
        breaks = jenkspy.jenks_breaks(data, k)
        # compute SSE:
        sse = 0
        for i in range(k):
            low, high = breaks[i], breaks[i+1]
            cls = [x for x in data if low <= x <= high]
            sse += np.var(cls, ddof=0) * len(cls)
        
        cost = sse + penalty * k
        # or for AIC: cost = 2*k + n * np.log(sse/n)
        if cost < best['cost']:
            best.update(k=k, cost=cost, breaks=breaks)
    
    return best

# Example usage:
if __name__ == "__main__":
    buckets = [
        (10, 40, 0.45),
        (70, 90, 0.45),
    ]
    sample_data = generate_weighted_samples(1000, buckets)
    
    # Quick check of bucket counts:
    from collections import Counter
    cnt = Counter(
        'A' if 10 <= x <= 40 else
        'B' if 70 <= x <= 90 else
        'other'
        for x in sample_data
    )

    print(cnt)
    
    print('-' * 40)
    
    data = pd.Series(sample_data)
    
    # Find optimal number of classes using Jenks natural breaks
    # 1. Prepare data
    X = np.array(data).reshape(-1,1)

    # 2. Run DBSCAN
    #    eps = maximum distance between two samples in the same neighborhood
    #    min_samples = min points to form a dense region
    db = DBSCAN(eps=2.5, min_samples=50).fit(X)
    # clusterer = hdbscan.HDBSCAN(min_cluster_size=200, min_samples=50, cluster_selection_epsilon=8.5)
    labels = db.labels_  # -1 means “noise”
    # labels = clusterer.fit_predict(X)

    # 3. Extract clusters
    clusters = {}
    for lbl in set(labels):
        if lbl == -1: continue
        clusters[lbl] = X[labels == lbl].flatten()

    # 4. Inspect
    total_samples = len(data)
    samples_captured = sum(len(pts) for pts in clusters.values())
    for lbl, pts in clusters.items():
        print(f"Cluster {lbl}: range {pts.min()}–{pts.max()}, size {len(pts)}")
    total_cluster_range = sum(pts.max() - pts.min() for pts in clusters.values()) 
    
    print(f"Noise points: {(labels==-1).sum()}")
    print(f"Captured {samples_captured} out of {total_samples} samples ({samples_captured/total_samples:.2%})")
    print(f"Total cluster range: {total_cluster_range}")
    
    # Failed attempt
    # optimal = find_optimal_k(data)
    
    # print(f"Optimal k={optimal['k']} with cost={optimal['cost']:.3f}, breaks={optimal['breaks']}")
    
    # Failed attempt
    # best_k, best_gvf, best_breaks = None, -1, None
    # max_k = 10   # or whatever upper bound makes sense
    # for k in range(2, max_k+1):
    #     breaks = jenkspy.jenks_breaks(data, k)

    #     # compute GVF for these breaks
    #     total_var = data.var(ddof=0)
    #     within_var = 0
    #     for i in range(k):
    #         # slice out class i
    #         low, high = breaks[i], breaks[i+1]
    #         cls = [x for x in data if low <= x <= high]
    #         within_var += np.var(cls, ddof=0) * len(cls)/len(data)
    #     gvf = (total_var - within_var) / total_var
    #     if gvf > best_gvf:
    #         best_gvf, best_k, best_breaks = gvf, k, breaks
            
    # print(f"Optimal k={best_k} with GVF={best_gvf:.3f}, breaks={best_breaks}")

