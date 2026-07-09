// Package grpcmeta defines standard gRPC metadata keys used for forwarding
// authenticated caller identity from the gateway to internal services.
package grpcmeta

const (
	UserIDHeader  = "x-user-id"
	IsAdminHeader = "x-caller-is-admin"
)
