package zssh

import (
	"errors"
	"fmt"
	"github.com/LeeZXin/zallet/internal/util"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"log"
)

type Server struct {
	*ssh.Server
}

type ServerOpts struct {
	Host             string
	HostKey          string
	PublicKeyHandler ssh.PublicKeyHandler
	SessionHandler   ssh.Handler
}

func NewServer(opts ServerOpts) (*Server, error) {
	if opts.PublicKeyHandler == nil {
		return nil, errors.New("nil publicKeyHandler")
	}
	if opts.SessionHandler == nil {
		return nil, errors.New("nil sessionHandler")
	}
	if opts.Host == "" {
		return nil, errors.New("wrong host")
	}
	hostKey, err := util.ReadOrGenRsaKey(opts.HostKey)
	srv := &ssh.Server{
		Addr:             opts.Host,
		PublicKeyHandler: opts.PublicKeyHandler,
		Handler:          opts.SessionHandler,
		ServerConfigCallback: func(ctx ssh.Context) *gossh.ServerConfig {
			config := &gossh.ServerConfig{
				Config: gossh.Config{
					KeyExchanges: keyExchanges,
					MACs:         macs,
					Ciphers:      ciphers,
				},
			}
			return config
		},
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
	}
	if err = srv.SetOption(ssh.HostKeyFile(hostKey)); err != nil {
		return nil, fmt.Errorf("set host key failed: %v", err)
	}
	go func() {
		log.Printf("start ssh server %s", opts.Host)
		err2 := srv.ListenAndServe()
		if err2 != nil && err2 != ssh.ErrServerClosed {
			log.Fatalf("start ssh server err: %v", err)
		}
	}()
	return &Server{
		Server: srv,
	}, nil
}
