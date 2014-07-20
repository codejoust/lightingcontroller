package main


import (
	"github.com/tarm/goserial"
	"github.com/gorilla/mux"
	"net/http"
	"flag"
	"log"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
	"fmt"
	"encoding/json"
)

var serialPort = flag.String("serial", "/dev/master", "Serial Port Path to Use")
var webPort = flag.Int("port", 8080, "Webserver port to use")
var useSerial = flag.Bool("enableserial", true, "Use serial?")

type PowerDevice struct {
	Name string `json:"name"`
	OnChannel int `json:"on"`
	OffChannel int `json:"off"`
	PoweredState bool `json:"powered_state"`
}

type ActionDevice struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Name string `json:"name"`
}

var ConfigJson struct {
	PowerDevices []PowerDevice `json:"power_devices"`
	ActionDevices []ActionDevice `json:"action_devices"`
}

var SerialPort io.ReadWriteCloser

func connectSerial() {
	c := &serial.Config{Name: *serialPort, Baud: 19200}
    s, err := serial.OpenPort(c)
    if err != nil {
    	log.Fatal(err)
    }
    SerialPort = s
}

func homePageHandler(w http.ResponseWriter, r *http.Request){
	http.ServeFile(w, r, "views/index.html")
}

func findDevice(action string) *PowerDevice {
	for _, el := range ConfigJson.PowerDevices {
		if el.Name == action {
			return &el
		}
	}
	return nil
}

func findAction(action string) *ActionDevice {
	for _, el := range ConfigJson.ActionDevices {
		if el.Name == action {
			return &el
		}
	}
	return nil
}

func sendDeviceSignal(OnChannel int) {
	if SerialPort != nil {
		num, err := fmt.Fprintf(SerialPort, "%dc\n", OnChannel)
		if err != nil {
			fmt.Printf("ERROR WRITING TO SERIAL PORT\n")
		} else {
			fmt.Printf("Write %d bytes to serial port.\n", num)
		}
	} else {
		fmt.Printf("Didn't connect to serial port.\n")
	}
}

func turnOnDevice(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	device_name := vars["device"]
	device := findDevice(device_name)
	if device == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
		return
	}
	sendDeviceSignal(device.OnChannel)
	fmt.Fprintf(w, "OK", device.Name);
	fmt.Printf("[device_change]: %s turning on.\n", device_name);
}

func turnOffDevice(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	device_name := vars["device"]
	device := findDevice(device_name)
	if device == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
		return
	}
	sendDeviceSignal(device.OffChannel)
	fmt.Fprintf(w, "OK", device.Name);
	fmt.Printf("[device_change]: %s turning off.\n", device_name);
}

func runCommand(action_info *ActionDevice, req *http.Request) {
	val, err := strconv.Atoi(req.FormValue("val"))
	if err == nil {
		cmd := exec.Command("sh", "-c", "scripts/" + action_info.Path, strconv.Itoa(val))
		cmd.Start()
	} else {
		cmd := exec.Command("sh", "-c", "scripts/" + action_info.Path, "")
		cmd.Start()
	}
}

func performAction(w http.ResponseWriter, req *http.Request) {
	action := mux.Vars(req)["action"]
	action_info := findAction(action)
	if action_info == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
		return
	}
	if action_info.Type == "script" {
		go runCommand(action_info, req)
	} else {
		http.Error(w, "Action not implemented yet.", http.StatusNotFound)
		return
	}
	fmt.Printf("Action %s has been completed.\n", action_info.Name)
	fmt.Fprintf(w, "OK")
}

func listActions(w http.ResponseWriter, req *http.Request) {
	enc := json.NewEncoder(w)
	if err := enc.Encode(&ConfigJson.ActionDevices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func queryAllLightState(w http.ResponseWriter, req *http.Request) {
	enc := json.NewEncoder(w)
	if err := enc.Encode(&ConfigJson.PowerDevices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func queryLightState(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	action := vars["device"]
	device := findDevice(action)
	if device == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setupMuxes() {
	r := mux.NewRouter()
	r.HandleFunc("/", homePageHandler)
	r.HandleFunc("/devices/power/{device}/on", turnOnDevice)
	r.HandleFunc("/devices/power/{device}/off", turnOffDevice)
	r.HandleFunc("/actions/{action}", performAction)
	r.HandleFunc("/actions", listActions)
	r.HandleFunc("/devices/power/{device}", queryLightState)
	r.HandleFunc("/devices/power", queryAllLightState)
	http.Handle("/", r)
}

func readConfigFile() {
	configFile, err := os.Open("conf.json")
	if err != nil {
		log.Fatal("Cannot Read Config Json%s\n", err);
	}
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&ConfigJson); err != nil {
		log.Fatal("Cannot Read Config Json%s\n", err);
	}
	fmt.Printf("Successfully read JSON file.\n")
}

func setupLightState() {
	// turn off all lights
	for _, el := range ConfigJson.PowerDevices {
		sendDeviceSignal(el.OffChannel)
		el.PoweredState = false
		time.Sleep(100)
	}
}


func main(){
	flag.Parse()
	readConfigFile()
	if *useSerial {
		connectSerial()
	}
	setupLightState()
	setupMuxes()
	fmt.Printf("Setup paths.\n")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webPort), nil))
}




