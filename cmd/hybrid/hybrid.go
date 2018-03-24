package main

import (
	"github.com/empirefox/hybrid/hybridclient"
	"go.uber.org/zap"
)

// tox-account
//{
//  "Address": "6D5897C4DC7D5322406DB684436BBCA832630A6D717A9FD1E3FCDC5D9196296E295A4602424D",
//  "Secret": "9ED64C8A36138F0E100522AE5EC4D4369F487D5E149199756953D299ACBB26F5",
//  "Pubkey": "6D5897C4DC7D5322406DB684436BBCA832630A6D717A9FD1E3FCDC5D9196296E",
//  "Nospam": 693782018
//}

func main() {
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	config, err := hybridclient.LoadConfig()
	if err != nil {
		log.Fatal("LoadConfig", zap.Error(err))
	}

	client, err := hybridclient.NewClient(*config, nil, log)
	if err != nil {
		log.Fatal("NewClient", zap.Error(err))
	}

	err = client.Run()
	if err != nil {
		log.Fatal("Run", zap.Error(err))
	}
}
