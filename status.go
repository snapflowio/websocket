package websocket

import "github.com/coder/websocket"

type Status = websocket.StatusCode

const (
	StatusNormalClosure           Status = websocket.StatusNormalClosure
	StatusGoingAway               Status = websocket.StatusGoingAway
	StatusProtocolError           Status = websocket.StatusProtocolError
	StatusUnsupportedData         Status = websocket.StatusUnsupportedData
	StatusNoStatusRcvd            Status = websocket.StatusNoStatusRcvd
	StatusAbnormalClosure         Status = websocket.StatusAbnormalClosure
	StatusInvalidFramePayloadData Status = websocket.StatusInvalidFramePayloadData
	StatusPolicyViolation         Status = websocket.StatusPolicyViolation
	StatusMessageTooBig           Status = websocket.StatusMessageTooBig
	StatusMandatoryExtension      Status = websocket.StatusMandatoryExtension
	StatusInternalError           Status = websocket.StatusInternalError
	StatusServiceRestart          Status = websocket.StatusServiceRestart
	StatusTryAgainLater           Status = websocket.StatusTryAgainLater
	StatusBadGateway              Status = websocket.StatusBadGateway
	StatusTLSHandshake            Status = websocket.StatusTLSHandshake
)

type CloseSource int

const (
	ClientCloseSource CloseSource = iota
	ServerCloseSource
)
