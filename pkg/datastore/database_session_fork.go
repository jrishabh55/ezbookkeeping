package datastore

// Omit excludes the specified columns from the query
func (s *XOrmSession) Omit(columns ...string) DatabaseSession {
	s.Session.Omit(columns...)
	return s
}

// AllCols includes all columns, including zero values, in an update
func (s *XOrmSession) AllCols() DatabaseSession {
	s.Session.AllCols()
	return s
}

// Join joins another table
func (s *XOrmSession) Join(joinOperator string, tablename any, condition any, args ...any) DatabaseSession {
	s.Session.Join(joinOperator, tablename, condition, args...)
	return s
}

// SumInt sums the specified integer column
func (s *XOrmSession) SumInt(bean any, columnName string) (int64, error) {
	return s.Session.SumInt(bean, columnName)
}
