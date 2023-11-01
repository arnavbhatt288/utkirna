package main

func main() {
	if isPermAvailable() {
		StartGui()
	} else {
		HandleStartError()
	}
}
