package optvalidation

// RegimeConfidenceFilter drops every TrainingRow whose RegimeConfidence is
// strictly less than cfg.RegimeConfidenceThreshold, returning the surviving
// rows and the dropped rows separately. A row exactly at the threshold
// survives (strict less-than semantics).
func RegimeConfidenceFilter(rows []TrainingRow, cfg Config) (kept, dropped []TrainingRow) {
	kept = make([]TrainingRow, 0, len(rows))
	dropped = make([]TrainingRow, 0)

	for _, r := range rows {
		if r.RegimeConfidence < cfg.RegimeConfidenceThreshold {
			dropped = append(dropped, r)
			continue
		}
		kept = append(kept, r)
	}

	return kept, dropped
}
