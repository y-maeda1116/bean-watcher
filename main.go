package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"bean-watcher/internal/clock"
	"bean-watcher/internal/discord"
	"bean-watcher/internal/model"
	"bean-watcher/internal/notify"
	"bean-watcher/internal/record"
	"bean-watcher/internal/store"
)

const (
	configPath = "data/config.json"
	dataPath   = "data/data.json"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: bean-watcher <record|notify> [args]")
	}
	switch os.Args[1] {
	case "record":
		runRecord(os.Args[2:])
	case "notify":
		runNotify(os.Args[2:])
	default:
		log.Fatalf("unknown subcommand: %s", os.Args[1])
	}
}

func runRecord(args []string) {
	if len(args) < 1 {
		log.Fatal("usage: bean-watcher record <pour|clean|buy> ...")
	}
	sub := args[0]
	c := clock.Real{}
	cfg := mustLoadConfig(configPath)
	d := mustLoadData(dataPath)
	today := jstToday(c)
	d = record.Prune(d, cfg.AvgWindowDays, today)

	var err error
	switch sub {
	case "pour":
		fs := flag.NewFlagSet("pour", flag.ExitOnError)
		cups := fs.Int("cups", 1, "number of cups")
		fs.Parse(args[1:])
		d, err = record.PourCoffee(d, cfg, today, *cups)
	case "clean":
		fs := flag.NewFlagSet("clean", flag.ExitOnError)
		target := fs.String("target", "", "descaling|grinder")
		fs.Parse(args[1:])
		d, err = record.Clean(d, today, *target)
	case "buy":
		fs := flag.NewFlagSet("buy", flag.ExitOnError)
		grams := fs.Int("grams", 0, "grams purchased")
		fs.Parse(args[1:])
		if *grams <= 0 {
			log.Fatalf("invalid grams: must be positive, got %d", *grams)
		}
		d, err = record.AddBeans(d, cfg, today, *grams)
	default:
		log.Fatalf("unknown record action: %s", sub)
	}
	if err != nil {
		log.Fatalf("record: %v", err)
	}
	mustSaveData(dataPath, d)
}

func runNotify(args []string) {
	c := clock.Real{}
	cfg := mustLoadConfig(configPath)
	d := mustLoadData(dataPath)
	today := jstToday(c)
	d = record.Prune(d, cfg.AvgWindowDays, today)

	cur := notify.CurrentLevels(d, cfg, today)
	diff := notify.ComputeDiff(d.NotifyState, cur)

	if msg := notify.BuildMessage(d, cfg, cur, diff, today); msg != "" {
		webhook := os.Getenv("DISCORD_WEBHOOK_URL")
		if err := discord.Send(context.Background(), webhook, msg); err != nil {
			// 送信失敗時は notify_state を更新せず終了（次回リトライ）
			log.Fatalf("notify send: %v", err)
		}
	}

	// notify_state 更新（未設定のメンテナンスは元の値を維持）
	ns := model.NotifyState{
		Beans:     cur.Beans,
		Descaling: cur.Descaling,
		Grinder:   cur.Grinder,
	}
	if ns.Descaling == "" {
		ns.Descaling = d.NotifyState.Descaling
	}
	if ns.Grinder == "" {
		ns.Grinder = d.NotifyState.Grinder
	}
	d.NotifyState = ns
	mustSaveData(dataPath, d)
}

func jstToday(c clock.Clock) string {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		loc = time.UTC
	}
	return c.Now().In(loc).Format("2006-01-02")
}

func mustLoadConfig(path string) model.Config {
	c, err := store.LoadConfig(path)
	if err != nil {
		log.Fatalf("load config %s: %v", path, err)
	}
	return c
}

func mustLoadData(path string) model.Data {
	d, err := store.LoadData(path)
	if err != nil {
		log.Fatalf("load data %s: %v", path, err)
	}
	return d
}

func mustSaveData(path string, d model.Data) {
	if err := store.SaveData(path, d); err != nil {
		log.Fatalf("save data %s: %v", path, err)
	}
}
