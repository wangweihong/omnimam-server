package errors

func Message(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
