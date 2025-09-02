package job

// hertzbeat Collect Job related types

// Job represents a complete monitoring job
type Job struct {
	ID                  int64             `json:"id"`
	TenantID            int64             `json:"tenantId"`
	MonitorID           int64             `json:"monitorId"`
	Metadata            map[string]string `json:"metadata"`
	Labels              map[string]string `json:"labels"`
	Annotations         map[string]string `json:"annotations"`
	Hide                bool              `json:"hide"`
	Category            string            `json:"category"`
	App                 string            `json:"app"`
	Name                map[string]string `json:"name"`
	Help                map[string]string `json:"help"`
	HelpLink            map[string]string `json:"helpLink"`
	Timestamp           int64             `json:"timestamp"`
	DefaultInterval     int64             `json:"defaultInterval"`
	Intervals           []int64           `json:"intervals"`
	IsCyclic            bool              `json:"isCyclic"`
	Params              []ParamDefine     `json:"params"`
	Metrics             []Metrics         `json:"metrics"`
	Configmap           []Configmap       `json:"configmap"`
	IsSd                bool              `json:"isSd"`
	PrometheusProxyMode bool              `json:"prometheusProxyMode"`

	// Internal fields
	EnvConfigmaps    map[string]Configmap `json:"-"`
	DispatchTime     int64                `json:"-"`
	PriorMetrics     []Metrics            `json:"-"`
	ResponseDataTemp []MetricsData        `json:"-"`
}
