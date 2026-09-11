package util

func ConvertBoolToInt(b bool) int {
	switch b {
	case true:
		return 1
	case false:
		return 0
	default:
		return -1
	}
}
