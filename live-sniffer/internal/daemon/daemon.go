package daemon

import (
	"context"
	"encoding/json"
	"live-sniffer/internal/pipeline"
	"live-sniffer/internal/proto"
	"live-sniffer/internal/storage"
	"log/slog"
	"net"
	"os"
)

type Daemon struct {
    SocketPath string
    Launcher   *pipeline.Launcher
    Store      *storage.Store
    cancel     context.CancelFunc
}

func (d *Daemon) Start(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)
	os.Remove(d.SocketPath)
	l, err := net.Listen("unix", d.SocketPath)
	if err != nil{
		slog.Error("There was an error connecting to the sock file by the daemon", "error", err)
		return err
	}
	defer l.Close()
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				slog.Error("error accepting connection", "error", err)
				continue
			}
		}
		go d.handleConn(ctx, conn)
	}
	return nil
}

func (d *Daemon) handleConn(ctx context.Context, conn net.Conn) error{
	defer conn.Close()
	var req proto.Request
	err := json.NewDecoder(conn).Decode(&req)
	if err != nil{
		slog.Error("There was an error reading from the connection", "error", err)
		return err
	}
	resp, err := d.handleRequest(ctx, req)
	if err != nil {
		slog.Error("There was an error reading from the connection", "error", err)
		return err
	}
	err = json.NewEncoder(conn).Encode(&resp)
	return nil
}

func (d *Daemon) handleRequest(ctx context.Context, req proto.Request) (proto.Response, error){
	var err error
	var r proto.Response
	switch req.Command{
	case proto.CmdStart:
		sID, err := d.Launcher.StartPipeline(ctx, req.Interface, req.Workers)
		if err != nil{
			r.OK = false
			r.Error = err.Error()
		} else{
			r.OK = true
			r.SessionID = sID
		}
	case proto.CmdStop:
		err = d.Launcher.StopPipeline(ctx, req.SessionID)
		if err != nil{
			r.OK = false
			r.Error = err.Error()
		} else{
			r.OK = true
			r.SessionID = req.SessionID
		}
	case proto.CmdShutdown:
		d.Launcher.StopAll(ctx)
		d.cancel()
		r.OK = true
	case proto.CmdStatus:
		p, err := d.Launcher.ViewPipelines(ctx)
		if err != nil{
			r.OK = false
			r.Error = err.Error()
		} else{
			r.OK = true
			r.Sessions = p
		}
	}
	return r, nil
}

