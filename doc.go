// Package nq (nanoQ) is a minimalistic brokerless Pub-Sub message queue for streaming.
// An application that wishes to receive messages should embed nq.Sub,
// and it will bind to a port and read network traffic.
// Another application can send messages to the first one using nq.Pub
//
// nanoQ is designed to lose messages in a scenario when subscribers are inaccessible or
// not able to keep up with the workload. The aim is to transfer audio, video, or clicks,
// where it is important to get fresh data with minimal delay.
package nq
