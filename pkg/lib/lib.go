package lib

func IsTruthy(value string) bool {
	switch value {
	case "true", "t", "yes", "y", "1", "on":
		return true
	default:
		return false
	}
}

func IsTruthyBit(value string) int {
	switch value {
	case "true", "t", "yes", "y", "1", "on":
		return 1
	default:
		return 0
	}
}
