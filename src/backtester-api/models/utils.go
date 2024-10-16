package models

// CalculateMaintenanceRequirement calculates the maintenance requirement based on stock price and shares sold short
func calculateMaintenanceRequirement(stockQuantity, stockPrice float64) float64 {
	if stockQuantity > 0 {
		return stockQuantity * stockPrice
	} else if stockQuantity < 0 {
		sharesSoldShort := -stockQuantity

		var maintenanceRequirementPerShare float64

		// Determine maintenance requirement based on stock price
		if stockPrice >= 5.0 {
			// For stocks trading at $5 or higher
			maintenanceRequirementPerShare = max(1.50*stockPrice, 5.0)
		} else {
			// For stocks trading below $5
			maintenanceRequirementPerShare = max(stockPrice, 2.5)
		}

		// Total maintenance requirement
		totalMaintenance := maintenanceRequirementPerShare * sharesSoldShort
		return totalMaintenance
	}

	return 0
}
