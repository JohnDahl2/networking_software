package main

func main() {
	ConnectDB()
    defer DB.Close()

}