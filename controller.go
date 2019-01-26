package main

import (
  "fmt"
  "net/http"
  "encoding/json"
  "time"
  "strconv"
  auth "github.com/abbot/go-http-auth"
)

func main() {
  loadConfig(true)
  myClient = http.Client{}
  go func() {
    loops := 0
    for{
      fmt.Println("Staying Alive for ", strconv.Itoa(loops * 10), " minutes.")
      loops++
      time.Sleep(time.Minute * 10)
    }
  }()
  htmlInterface()
}

func htmlInterface(){

  monitorOutput.unshift("AdPusher is ready.")
  monitorOutput.unshift("PAUSE")

  http.HandleFunc("/controller", auth.JustCheck(authenticator, controllerHandle))
  http.HandleFunc("/send", auth.JustCheck(authenticator, sendHandle))
  http.HandleFunc("/monitor", auth.JustCheck(authenticator, monitorHandle))
  http.HandleFunc("/abort", auth.JustCheck(authenticator, abortHandle))

  fmt.Println("listening on", configVars["localhost"])
  err := http.ListenAndServe(configVars["localhost"], nil)
  fmt.Println(err)
}

func controllerHandle(w http.ResponseWriter, r *http.Request){
  http.ServeFile(w, r, configVars["serverRoot"] + "html/controller.html")
}

func sendHandle(w http.ResponseWriter, r *http.Request) {
  if locked == false {
    mode = r.URL.Query()["mode"][0]
    prop = r.URL.Query()["prop"][0]
    doTheThings()
  } else {
    monitorOutput.unshift("Sender is running and cannot receive new commands.")
  }
}

func monitorHandle(w http.ResponseWriter, r *http.Request){
  json.NewEncoder(w).Encode(`{"message":"` + monitorOutput.pop() + `"}`)
}

func abortHandle(w http.ResponseWriter, r *http.Request){
  abort = true
  monitorOutput.unshift("Process will abort.")
}
