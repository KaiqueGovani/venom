package db

import (
	"github.com/couchbase/gocb/v2"
)

func Connect() (*gocb.Cluster, error) {
	// Update this to your cluster details

	options := gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: username,
			Password: password,
		},
	}

	// Sets a pre-configured profile called "wan-development" to help avoid latency issues
	// when accessing Capella from a different Wide Area Network
	// or Availability Zone (e.g. your laptop).
	if err := options.ApplyProfile(gocb.ClusterConfigProfileWanDevelopment); err != nil {
		return nil, err
	}

	// Initialize the Connection
	cluster, err := gocb.Connect("couchbases://"+connectionString, options)
	if err != nil {
		return nil, err
	}

	return cluster, nil
}
