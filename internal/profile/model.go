package profile

type OnError string

const (
	OnErrorContinue OnError = "continue"
	OnErrorStop     OnError = "stop"
)

type TransactionMode string

const (
	TransactionAutoCommit    TransactionMode = "auto_commit"
	TransactionRunnerManaged TransactionMode = "transaction"
	TransactionScriptManaged TransactionMode = "script_managed"
)

type Connection struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

type Execution struct {
	OnError         OnError         `yaml:"on_error" json:"onError"`
	TransactionMode TransactionMode `yaml:"transaction_mode" json:"transactionMode"`
}

type Script struct {
	ID              string          `yaml:"id" json:"id"`
	Name            string          `yaml:"name" json:"name"`
	File            string          `yaml:"file" json:"file"`
	Enabled         bool            `yaml:"enabled" json:"enabled"`
	Order           int             `yaml:"order" json:"order"`
	TransactionMode TransactionMode `yaml:"transaction_mode,omitempty" json:"transactionMode,omitempty"`
}

type Profile struct {
	ID         string     `yaml:"id" json:"id"`
	Name       string     `yaml:"name" json:"name"`
	Version    int        `yaml:"version" json:"version"`
	Connection Connection `yaml:"connection" json:"connection"`
	Execution  Execution  `yaml:"execution" json:"execution"`
	Scripts    []Script   `yaml:"scripts,omitempty" json:"scripts"`
}
