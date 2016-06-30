package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/xtfly/gofd/agent"
	"github.com/xtfly/gofd/common"
	"github.com/xtfly/gofd/server"
	"github.com/xtfly/gokits"
)

var (
	a = flag.Bool("a", false, "start as a agent")
	s = flag.Bool("s", false, "start as a server")
	p = flag.String("p", "", "create a password encrypted by AES128")
)

func usage() {
	fmt.Println("gofd [<-a|-s> <configfile>] [-p <passwd>]")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Parse()
	if !*a && !*s && *p == "" {
		fmt.Println("miss option")
		usage()
	}

	if *p != "" {
		factor := gokits.NewRand(8)
		crc := gokits.KermitStr(factor)
		crypto, _ := gokits.NewCrypto(factor, crc)
		stxt, _ := crypto.EncryptStr(*p)
		fmt.Println("factor =", factor)
		fmt.Println("crc =", crc)
		fmt.Println("stxt =", stxt)
		return
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

	if *a {
		if svc, err = agent.NewAgent(cfg); err != nil {
			fmt.Printf("start agent error, %s.\n", err.Error())
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
