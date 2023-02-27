package cics

type HeaderV2 struct {
	ABIBanca           string `mainframe:"start=1,length=5"`
	ProgramName        string `mainframe:"start=6,length=8" validate:"required"`
	Conversion         string `mainframe:"start=14,length=1"`
	FlagDebug          string `mainframe:"start=15,length=1"`
	LogLevel           string `mainframe:"start=16,length=1"`
	CicsResponseCode   string `mainframe:"start=17,length=3"`
	CicsResponse2Code  string `mainframe:"start=20,length=3"`
	CicsAbendCode      string `mainframe:"start=23,length=4"`
	ErrorDescription   string `mainframe:"start=27,length=30"`
	RequestIdClient    string `mainframe:"start=57,length=255"`
	CorellationIdPoste string `mainframe:"start=312,length=100"`
	RequestIdLegacy    string `mainframe:"start=412,length=100"`
	TransId            string `mainframe:"start=512,length=4"`
}

type HeaderV3 struct {
	Version          string `mainframe:"start=1,length=6"`
	ProgramName      string `mainframe:"start=7,length=8"`
	TransId          string `mainframe:"start=15,length=4"`
	SystemId         string `mainframe:"start=19,length=8"`
	Canale           string `mainframe:"start=27,length=8"`
	Conversion       string `mainframe:"start=35,length=1"`
	FlagDebug        string `mainframe:"start=36,length=1"`
	LogLevel         string `mainframe:"start=37,length=1"`
	RollBack         string `mainframe:"start=38,length=1"`
	ReturnCode       string `mainframe:"start=39,length=5"`
	ErrorDescription string `mainframe:"start=44,length=30"`
	FlagCheckIdem    string `mainframe:"start=74,length=1"`
	CicsTargetPool   string `mainframe:"start=75,length=4"`
	TrackingId       string `mainframe:"start=79,length=100"`
	RequestId        string `mainframe:"start=179,length=100"`
	SubRequestId     string `mainframe:"start=279,length=12"`
}
