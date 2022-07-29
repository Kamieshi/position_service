// Package model
package model

import "time"

// Price struct priceStorage
type Price struct {
	Ask  uint32    `protobuf:"varint,2,opt,name=Ask,proto3" json:"Ask,omitempty"`
	Bid  uint32    `protobuf:"varint,3,opt,name=Bid,proto3" json:"Bid,omitempty"`
	Time time.Time `protobuf:"bytes,4,opt,name=Time,proto3" json:"Time,omitempty"`
}
