package daemon

import (
	"encoding/json"
	"live-sniffer/internal/proto"
	"log/slog"
	"net"
)

func Send(socketPath string, req proto.Request) (proto.Response, error){
	c, err := net.Dial("unix", socketPath)
	if err != nil{
		slog.Error("There was an error connecting to the sock file", "error", err)
		return proto.Response{
			OK: false,
			Error: err.Error(),
		}, err
	}
	defer c.Close()
	err = json.NewEncoder(c).Encode(req)
	if err != nil {
		slog.Error("There was an issue with writing to sock", "error", err)
		return proto.Response{
			OK: false,
			Error: err.Error(),
		}, err 
	}
	var resp proto.Response
	err = json.NewDecoder(c).Decode(&resp)
	if err != nil{
		slog.Error("There was an issue with decoding infomration from the sock", "error", err)
		return proto.Response{
			OK: false,
			Error: err.Error(),
		}, err
	}
	return resp, nil
}