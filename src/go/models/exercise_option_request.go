package models

type ExerciseOptionRequest struct {
	Order            any // todo: change this to orderRecord when we migrate
	AssignedQuantity float64
	AssignmentPrice  float64
}
