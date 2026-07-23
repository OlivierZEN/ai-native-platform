package governance

const (
	DefaultFieldsPerObject    = 300
	MaxFieldsPerObject        = 500
	DefaultIndexedFields8GiB  = 10
	MaxIndexedFields8GiB      = 20
	DefaultIndexedFields16GiB = 20
	MaxIndexedFields16GiB     = 40
	AbsoluteIndexedFields     = 50
	MaxRecordJSONBytes        = 256 * 1024
	RecordJSONWarningBytes    = 64 * 1024
	MaxJSONFieldBytes         = 64 * 1024
	MaxJSONDepth              = 8
	MaxJSONArrayElements      = 1000
)
