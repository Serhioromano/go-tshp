package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/Valentin-Kaiser/go-dbase/dbase"
	"github.com/simonvetter/modbus"
	"github.com/urfave/cli/v3"
	"golang.org/x/text/encoding/charmap"
)

const dbFile = "./TEST22.DBF"

// const dbEncode = "UTF-8"
const interval = 1 * time.Second
const mbWriteFlagAddress = 10
const mbWeightAddress = 10  // Wheight before
const mbWeightAddress2 = 11 // Wheight after

// Адреса клиентов Modbus для опроса
var mbCalls = [...]int16{1, 2}

type Entry struct {
	ADR     int64     `dbase:"ADR"`
	WBBFORE int64     `dbase:"WBBFORE"`
	WBAFTER int64     `dbase:"WBAFTER"`
	WBTOTAL int64     `dbase:"WBTOTAL"`
	DATEW   time.Time `dbase:"DATEW"`
	TIMEW   string    `dbase:"TIMEW"`
	ACT     int64     `dbase:"ACT"`
}

func main() {

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "log",
				Usage: "Save errors to log file",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "stats",
				Usage:  "Read stats from DBF file",
				Action: stats,
			},
			{
				Name:   "start",
				Usage:  "Start windows service",
				Flags:  start_flags(),
				Action: start,
			},
			{
				Name:   "create",
				Usage:  "Create a DBF file",
				Action: createdb,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func start_flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "com",
			Usage:    "Com port. For example COM7",
			Required: true,
			Validator: func(v string) error {
				r, _ := regexp.Compile("^(COM)[0-9]{1,2}$")
				if r.MatchString(v) {
					return nil
				}
				return errors.New("parameter --com port does not mutch and should be COM1 or COM2 to COM99")
			},
		},
		&cli.StringFlag{
			Name:        "br",
			Usage:       "Com port boudrate 9600|14400|19200|38400.",
			DefaultText: "9600",
			Validator: func(v string) error {
				r, _ := regexp.Compile("^(9600|14400|19200|38400)$")
				if r.MatchString(v) {
					return nil
				}
				return errors.New("parameter --rb boudrate does not mutch and should be 9600|14400|19200|38400")
			},
		},
		&cli.StringFlag{
			Name:        "parity",
			Usage:       "Com port Parity mode. N, E or O.",
			DefaultText: "N",
			Validator: func(v string) error {
				r, _ := regexp.Compile("^(N|E|O)$")
				if r.MatchString(v) {
					return nil
				}
				return errors.New("parameter --parity parity does not mutch and should be E|O|N")
			},
		},
	}
}

func start(ctx context.Context, cmd *cli.Command) error {
	f, err := os.OpenFile("./error_log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	if cmd.Bool("log") {
		log.SetOutput(f)
	}

	var mbWheightBefore uint16
	var mbWheightAfter uint16

	log.Print(cmd.String("parity"))
	br, err := strconv.ParseUint(cmd.String("br"), 0, 32)
	if err != nil {
		log.Fatal(err)
	}
	arr := map[string]uint{
		"E": modbus.PARITY_EVEN,
		"O": modbus.PARITY_ODD,
		"N": modbus.PARITY_NONE,
	}

	client, err := modbus.NewClient(&modbus.ClientConfiguration{
		URL:      "rtu://" + cmd.String("com"),
		Speed:    uint(br),
		DataBits: 8,
		Parity:   arr[cmd.String("parity")],
		StopBits: 1,
		Timeout:  300 * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}
	err = client.Open()
	if err != nil {
		log.Fatal(err)
	}

	createdb(ctx, cmd)

	uptimeTicker := time.NewTicker(interval)
	defer uptimeTicker.Stop()

	quit := make(chan bool)

	fmt.Println("Start monitoring!")
	for {
		select {
		case <-uptimeTicker.C:
			for i := 0; i < 2; i++ {
				client.SetUnitId(uint8(mbCalls[i]))

				writeFlag, err := client.ReadCoil(mbWriteFlagAddress)
				if err != nil {
					log.Print(err, fmt.Sprintf(" Client ID: %d", mbCalls[i]))
					continue
				}
				if !bool(writeFlag) {
					continue
				}

				if mbWheightBefore, err = client.ReadRegister(mbWeightAddress, modbus.HOLDING_REGISTER); err != nil {
					log.Print(err)
					continue
				}
				if mbWheightAfter, err = client.ReadRegister(mbWeightAddress2, modbus.HOLDING_REGISTER); err != nil {
					log.Print(err)
					continue
				}

				table, err := dbase.OpenTable(&dbase.Config{
					Filename:   dbFile,
					TrimSpaces: true,
					WriteLock:  true,
					Untested:   true,
				})
				if err != nil {
					log.Print(err)
					continue
				}

				e := Entry{
					ADR:     int64(mbCalls[i]),
					WBBFORE: int64(mbWheightBefore),
					WBAFTER: int64(mbWheightAfter),
					WBTOTAL: int64(mbWheightBefore - mbWheightAfter),
					DATEW:   time.Now(),
					TIMEW:   fmt.Sprintf("%02d:%02d:%02d", time.Now().Hour(), time.Now().Minute(), time.Now().Second()),
					ACT:     int64(0),
				}

				fmt.Printf("value: %v \n", mbCalls[i])
				fmt.Printf("value: %v \n", int64(mbCalls[i]))
				fmt.Printf("value: %v \n", e)

				row, err := table.RowFromStruct(e)
				if err != nil {
					log.Fatal(err, " 1")
				}

				// Add the new row to the database table.
				err = row.Write()
				if err != nil {
					log.Fatal(err, " 2")
				}

				table.Close()

				err = client.WriteCoil(mbWriteFlagAddress, false)
				if err != nil {
					log.Print(err)
					close(quit)
				}
			}

		case <-quit:
			uptimeTicker.Stop()
			return nil
		}
	}
}

func stats(ctx context.Context, cmd *cli.Command) error {
	f, err := os.OpenFile("./error_log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	if cmd.Bool("log") {
		log.SetOutput(f)
	}
	table, err := dbase.OpenTable(&dbase.Config{
		Filename:   "./RESULTS.DBF",
		TrimSpaces: true,
		WriteLock:  true,
		Untested:   true,
	})
	if err != nil {
		return err
	}
	fmt.Printf(
		"Last modified: %v \nColumns count: %v \nRecord count: %v \nFile size: %v \n",
		table.Header().Modified(0),
		table.Header().ColumnsCount(),
		table.Header().RecordsCount(),
		table.Header().FileSize(),
	)

	return nil
}

func columns() []*dbase.Column {
	adr, err := dbase.NewColumn("ADR", dbase.Numeric, 10, 0, false)
	if err != nil {
		panic(err)
	}

	before, err := dbase.NewColumn("WBBFORE", dbase.Numeric, 10, 0, false)
	if err != nil {
		panic(err)
	}

	after, err := dbase.NewColumn("WBAFTER", dbase.Numeric, 10, 0, false)
	if err != nil {
		panic(err)
	}

	total, err := dbase.NewColumn("WBTOTAL", dbase.Numeric, 10, 0, false)
	if err != nil {
		panic(err)
	}

	dt, err := dbase.NewColumn("DATEW", dbase.Date, 0, 0, false)
	if err != nil {
		panic(err)
	}

	tm, err := dbase.NewColumn("TIMEW", dbase.Character, 10, 0, false)
	if err != nil {
		panic(err)
	}

	action, err := dbase.NewColumn("ACT", dbase.Numeric, 10, 0, false)
	if err != nil {
		panic(err)
	}

	return []*dbase.Column{
		adr,
		before,
		after,
		total,
		dt,
		tm,
		action,
	}
}

func createdb(ctx context.Context, cmd *cli.Command) error {
	if _, err := os.Stat(dbFile); errors.Is(err, os.ErrNotExist) {
		log.Print("Create new DBF file")
		file, err := dbase.NewTable(
			dbase.FoxBasePlus,
			&dbase.Config{
				Filename:   dbFile,
				Converter:  dbase.NewDefaultConverter(charmap.Windows1250),
				TrimSpaces: true,
				Untested:   true,
			},
			columns(),
			64,
			nil,
		)
		if err != nil {
			return err
		}
		file.Close()
	}
	return nil
}

// func main2() {

// 	var args struct {
// 		comport string `arg:"required" default:"COM7"`
// 		log     bool
// 	}
// 	arg.MustParse(&args)

// 	fmt.Println(args.comport)

// 	var mbWheightBefore uint16
// 	var mbWheightAfter uint16

// 	fmt.Println("Start monitoring!")
// 	if logToFile {
// 		f, err := os.OpenFile("./error_log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
// 		if err != nil {
// 			log.Fatalf("error opening file: %v", err)
// 		}
// 		defer f.Close()
// 		log.SetOutput(f)
// 	}

// 	if _, err := os.Stat(dbFile); errors.Is(err, os.ErrNotExist) {
// 		file, err := dbase.NewTable(
// 			dbase.FoxPro,
// 			&dbase.Config{
// 				Filename:   dbFile,
// 				Converter:  dbase.NewDefaultConverter(charmap.Windows1250),
// 				TrimSpaces: true,
// 			},
// 			columns(),
// 			64,
// 			nil,
// 		)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		file.Close()
// 	}

// 	client, err := modbus.NewClient(&modbus.ClientConfiguration{
// 		URL:      dbComPort,
// 		Speed:    9600,               // default
// 		DataBits: 8,                  // default, optional
// 		Parity:   modbus.PARITY_NONE, // default, optional
// 		StopBits: 1,                  // default if no parity, optional
// 		Timeout:  300 * time.Millisecond,
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	err = client.Open()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	uptimeTicker := time.NewTicker(interval)
// 	defer uptimeTicker.Stop()

// 	quit := make(chan bool)

// 	for {
// 		select {
// 		case <-uptimeTicker.C:
// 			for i := 0; i < 2; i++ {
// 				client.SetUnitId(uint8(mbCalls[i]))

// 				writeFlag, err := client.ReadCoil(mbWriteFlagAddress)
// 				if err != nil {
// 					log.Print(err, fmt.Sprintf(" Client ID: %d", mbCalls[i]))
// 					continue
// 				}
// 				if !bool(writeFlag) {
// 					continue
// 				}

// 				if mbWheightBefore, err = client.ReadRegister(mbWeightAddress, modbus.HOLDING_REGISTER); err != nil {
// 					log.Print(err)
// 					continue
// 				}
// 				if mbWheightAfter, err = client.ReadRegister(mbWeightAddress2, modbus.HOLDING_REGISTER); err != nil {
// 					log.Print(err)
// 					continue
// 				}

// 				table, err := dbase.OpenTable(&dbase.Config{
// 					Filename:   dbFile,
// 					TrimSpaces: true,
// 					WriteLock:  true,
// 				})
// 				if err != nil {
// 					log.Print(err)
// 					continue
// 				}

// 				e := Entry{
// 					Adr:    int32(mbCalls[i]),
// 					Before: int32(mbWheightBefore),
// 					After:  int32(mbWheightAfter),
// 					Date:   time.Now(),
// 					Time:   time.Now(),
// 					Action: 0,
// 				}

// 				row, err := table.RowFromStruct(e)
// 				if err != nil {
// 					log.Fatal(err, " 1")
// 				}

// 				// Add the new row to the database table.
// 				err = row.Write()
// 				if err != nil {
// 					log.Fatal(err, " 2")
// 				}

// 				table.Close()

// 				err = client.WriteCoil(mbWriteFlagAddress, false)
// 				if err != nil {
// 					log.Print(err)
// 					close(quit)
// 				}

// 				fmt.Printf("value: %v \n", writeFlag)
// 				fmt.Printf("value: %v \n", bool(writeFlag))
// 				close(quit)

// 			}

// 		case <-quit:
// 			uptimeTicker.Stop()
// 			return
// 		}
// 	}
// }

// if _, err := os.Stat(dbFile); errors.Is(err, os.ErrNotExist) {
// 	newTable := godbf.New(dbEncode)

// 	newTable.AddNumberField("ADR", 4, 0)
// 	newTable.AddNumberField("WBBFORE", 4, 0)
// 	newTable.AddNumberField("WBAFTER", 4, 0)
// 	newTable.AddTextField("DATEW", 20)
// 	newTable.AddTextField("TIMEW", 16)
// 	newTable.AddNumberField("ACT", 4, 0)

// 	idx, err := newTable.AddNewRecord()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	fmt.Printf("New record: %d \n", idx)

// 	newTable.SetFieldValueByName(idx, "ADR", "0")
// 	newTable.SetFieldValueByName(idx, "ACT", "1")
// 	newTable.SetFieldValueByName(idx, "DATEW", "11/11/2024")
// 	newTable.SetFieldValueByName(idx, "TIMEW", "11:12:25")
// 	newTable.SetFieldValueByName(idx, "WBBFORE", "270")
// 	newTable.SetFieldValueByName(idx, "WBAFTER", "1025")

// 	godbf.SaveToFile(newTable, dbFile)
// }

// workTable, err := godbf.NewFromFile(dbFile, dbEncode);
// if err != nil {
//  	log.Fatal(err)
// }
// fmt.Printf("Number of records: %d \n", workTable.NumberOfRecords())

// workTable.AddNumberField("ADR", 4, 0)
// workTable.AddNumberField("WBBFORE", 4, 0)
// workTable.AddNumberField("WBAFTER", 4, 0)
// workTable.AddDateField("DATE")
// workTable.AddNumberField("ACT", 4, 0)

// idx, err := workTable.AddNewRecord()
// if err != nil {
// 	log.Fatal(err)
// }
// fmt.Printf("New record: %d \n", idx)
// workTable.SetFieldValueByName(idx, "ADR", "0")
// workTable.SetFieldValueByName(idx, "ACT", "1")

// godbf.SaveToFile(workTable, dbFile);
