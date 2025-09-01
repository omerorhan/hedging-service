package storage

const (
	Default        = "Default"
	Monthly        = "Monthly"
	ratesBackupKey = "hedging:rates_backup"
	termsBackupKey = "hedging:terms_backup"

	// Data versioning keys
	dataVersionKey = "hedging:data_version"
	leaderLockKey  = "hedging:leader_lock"
)
