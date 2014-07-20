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
	"bufio"
	"time"
	"fmt"
	"encoding/json"
)

var serialPort = flag.String("serial", "/dev/master", "Serial Port Path to Use")
var webPort = flag.Int("port", 8080, "Webserver port to use")

type PowerDevice struct {
	Name string `json:"name"`
	onChannel int `json:"on"`
	offChannel int `json:"off"`
	PoweredState bool `json:"powered_state"`
}

type ActionDevice struct {
	Type string `json:"path"`
	Path string `json:"type"`
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

func sendDeviceSignal(onChannel int) {
	if SerialPort != nil {
		_, err := fmt.Fprintf(SerialPort, "c%d\n", onChannel)
		if err != nil {
			fmt.Printf("ERROR WRITING TO SERIAL PORT\n")
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
	sendDeviceSignal(device.onChannel)
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
	sendDeviceSignal(device.offChannel)
	fmt.Fprintf(w, "OK", device.Name);
	fmt.Printf("[device_change]: %s turning off.\n", device_name);

}

func performAction(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	action_info := findAction(vars["action"])
	if action_info == nil {
		http.Error(w, "Can't find Device", http.StatusNotFound)
		return
	}
	if (action_info.Type == 'script') {
		val, err := strconv.Atoi(r.FormValue("val"))
		if err == nil {
			cmd := exec.Command("./scripts/" + action_info.Path, strconv.Itoa(val))
			cmd.Start()
		} else {
			cmd := exec.Command("./scripts/" + action_info.Path, "")
			cmd.Start()
		}
	} else {
		http.Error(w, "Action not implemented yet.", http.StatusNotFound)
	}
	fmt.Printf("Action %s has been completed.\n", action)
	fmt.Fprintf(w, "OK")
}

func listActions(w http.ResponseWriter, req *http.Request) {
	enc := json.NewEncoder(w)
	if err := enc.Encode(&ConfigJson.ActionDevice); err != nil {
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
		log.Fatal("Cannot Read Config Json\n");
	}
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&ConfigJson); err != nil {
		log.Fatal("Cannot Read Config Json\n");
	}
	fmt.Printf("Successfully read JSON file.\n")
}

func setupLightState() {
	// turn off all lights
	for _, el := range ConfigJson.PowerDevices {
		sendDeviceSignal(el.offChannel)
		el.PoweredState = false
		time.Sleep(100)
	}
}


func main(){
	flag.Parse()
	readConfigFile()
	connectSerial()
	setupLightState()
	setupMuxes()
	fmt.Printf("Setup paths.\n")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webPort), nil))
}




