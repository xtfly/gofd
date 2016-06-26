package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xtfly/gofd/client"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/server"
)

var (
	c = flag.Bool("c", false, "start as a client")
	s = flag.Bool("s", false, "start as a server")
)

func usage() {
	fmt.Println("gofd [OPTION] <configfile>")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Parse()
	if !*c && !*s {
		fmt.Println("miss option")
		usage()
	}

	if flag.NArg() < 1 {
		fmt.Println("miss config file")
		usage()
	}

	cfgfile := flag.Args()[0]
	var cfg *common.Config
	var err error
	if cfg, err = common.ParserConfig(cfgfile, *s); err != nil {
		fmt.Printf("parser config file %s error, %s.\n", cfgfile, err.Error())
		os.Exit(3)
	}

	var svc common.Service
	if *s {
		if svc, err = server.NewServer(cfg); err != nil {
			fmt.Printf("start server error, %s.\n", err.Error())
			os.Exit(4)
		}
	}

	if *c {
		if svc, err = client.NewClient(cfg); err != nil {
			fmt.Printf("start client error, %s.\n", err.Error())
			os.Exit(4)
		}
	}

	if err = svc.Start(); err != nil {
		fmt.Printf("Start service failed, %s.\n", err.Error())
		os.Exit(4)
	}

	quitChan := listenSigInt()
	select {
	case <-quitChan:
		fmt.Printf("got control-C")
		svc.Stop()
	}
}

func listenSigInt() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	return c
}
