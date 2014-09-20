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
	"errors"
)

var serialPort = flag.String("serial", "/dev/master", "Serial Port Path to Use")
var webPort = flag.Int("port", 8080, "Webserver port to use")
var useSerial = flag.Bool("enableserial", true, "Use serial?")

type PowerDevice struct {
	Name string `json:"name"`
	Type string `json:"type"`
	MaxValue int `json:"maxvalue"`
	Channel int `json:"channel"`
	OnChannel int `json:"on"`
	OffChannel int `json:"off"`
	PoweredState int `json:"powered_state"`
}

type ActionDevice struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Name string `json:"name"`
}

var ConfigJson struct {
	PowerDevices []PowerDevice `json:"devices"`
	ActionDevices []ActionDevice `json:"actions"`
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
func appJSHandler(w http.ResponseWriter, r *http.Request){
	http.ServeFile(w, r, "static/app.js")
}
func appCSSHandler(w http.ResponseWriter, r *http.Request){
	http.ServeFile(w, r, "static/app.css")
}


func findDevice(action string) *PowerDevice {
	for i := range ConfigJson.PowerDevices {
		if ConfigJson.PowerDevices[i].Name == action {
			return &ConfigJson.PowerDevices[i]
		}
	}
	return nil
}

func findAction(action string) *ActionDevice {
	for i := range ConfigJson.ActionDevices {
		if ConfigJson.ActionDevices[i].Name == action {
			return &ConfigJson.ActionDevices[i]
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


func sendSerialCommand(sigtype int, channel int, value int) error {
	_, err := fmt.Fprintf(SerialPort, "%d%c%dw\n", channel, sigtype, value)
	fmt.Printf("%d%c%dw\n", channel, sigtype, value)
	return err
}

func sendDeviceUpdate(device *PowerDevice, value int) error {
	switch device.Type {
	case "remote": {
		if (value == 1){
			return sendSerialCommand('r', device.OnChannel, 1)
		} else if (value == 0) {
			return sendSerialCommand('r', device.OffChannel, 1)
		}
		return errors.New("device: invalid binary value") // Invalid value.
	}
	case "local": {
		if (value == 1 || value == 0) {
			return sendSerialCommand('l', device.Channel, value)
		}
		return errors.New("device: invalid binary value") // Invalid value.
	}
	case "dmx": {
		if (value < 0 || value >= 256) {
			return errors.New("device: argument out of range") // Obviously out of range.
		}
		return sendSerialCommand('d', device.Channel, value)
	}
	default:
		return errors.New("device: type unknown") // Invalid type.
	return nil
	}
}

func updateDevice(w http.ResponseWriter, req *http.Request) {
	device_name := req.FormValue("device")
	device := findDevice(device_name)
	if device == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
		return
	}
	value, err := strconv.Atoi(req.FormValue("val"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = sendDeviceUpdate(device, value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	device.PoweredState = value
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"ok\": true}")

	fmt.Printf("[device_change]: %s = %d.\n", device_name, value);
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
	w.Header().Set("Content-Type", "application/json")
	if err := enc.Encode(&ConfigJson.ActionDevices); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func queryAllLightState(w http.ResponseWriter, req *http.Request) {
	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
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
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setupMuxes() {
	r := mux.NewRouter()

	// Deprecated! Please use newer versions.
	r.HandleFunc("/devices/power/{device}/on", turnOnDevice)
	r.HandleFunc("/devices/power/{device}/off", turnOffDevice)
	r.HandleFunc("/actions/{action}", performAction)
	r.HandleFunc("/devices/power/{device}", queryLightState)
	r.HandleFunc("/", homePageHandler);

	http.Handle("/", r)
}

func setupHttpHandlers() {

	// Newer versions :).
	http.HandleFunc("/devices/power", queryAllLightState)
	http.HandleFunc("/actions", listActions)
	http.HandleFunc("/devices/power/update", updateDevice)
	http.HandleFunc("/static/app.js", appJSHandler);
	http.HandleFunc("/static/app.css", appCSSHandler);
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

func updateMaxValue(device *PowerDevice) {
	switch device.Type {
	case "dmx":
		device.MaxValue = 255;
	case "local":
		device.MaxValue = 1;
	case "remote":
		device.MaxValue = 1;
	}
}

func setupLightState() {
	// turn off all lights
	for i := range ConfigJson.PowerDevices {
		device := &ConfigJson.PowerDevices[i]
		updateMaxValue(device)
		sendDeviceUpdate(device, 0)
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
	setupHttpHandlers()
	setupMuxes()
	fmt.Printf("Setup paths.\n")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webPort), nil))
}




