package store

func nullableCurriculumName(name string) any {
	if name == "" {
		return nil
	}
	return name
}
