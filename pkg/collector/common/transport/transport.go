package transport

type RemotingService interface {
	Start() error
	Shutdown() error
	isStart() error
}

type TransportClient interface {
	RemotingService

	// todo add
}
