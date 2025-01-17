package cicsservice

type HeaderV2 struct {
	ABIBanca           string `fixed:"1,5"`
	ProgramName        string `fixed:"6,13" validate:"required"`
	Conversion         string `fixed:"14,14"`
	FlagDebug          string `fixed:"15,15"`
	LogLevel           string `fixed:"16,16"`
	CicsResponseCode   string `fixed:"17,19"`
	CicsResponse2Code  string `fixed:"20,22"`
	CicsAbendCode      string `fixed:"23,26"`
	ErrorDescription   string `fixed:"27,56"`
	RequestIdClient    string `fixed:"57,311"`
	CorrelationIdPoste string `fixed:"312,411"`
	RequestIdLegacy    string `fixed:"412,511"`
	TransId            string `fixed:"512,515"`
}

type HeaderV3 struct {
	Version          string `fixed:"1,6"`
	ProgramName      string `fixed:"7,14"`
	TransId          string `fixed:"15,18"`
	SystemId         string `fixed:"19,26"`
	Canale           string `fixed:"27,34"`
	Conversion       string `fixed:"35,35"`
	FlagDebug        string `fixed:"36,36"`
	LogLevel         string `fixed:"37,37"`
	RollBack         string `fixed:"38,38"`
	ReturnCode       string `fixed:"39,43"`
	ErrorDescription string `fixed:"44,73"`
	FlagCheckIdem    string `fixed:"74,74"`
	CicsTargetPool   string `fixed:"75,78"`
	TrackingId       string `fixed:"79,178"`
	RequestId        string `fixed:"179,278"`
	SubRequestId     string `fixed:"279,290"`
}
