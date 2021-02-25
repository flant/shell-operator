package server

type Settings struct {
	ServiceName    string
	ServerCertPath string
	ServerKeyPath  string
	ClientCAPaths  []string
	ListenPort     string
	ListenAddr     string
}
