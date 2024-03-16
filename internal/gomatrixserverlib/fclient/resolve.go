/* Copyright 2016-2017 Vector Creations Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fclient

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
)

// ResolutionResult is a result of looking up a Matrix homeserver according to
// the federation specification.
type ResolutionResult struct {
	Destination    string          // The hostname and port to send federation requests to.
	Host           spec.ServerName // The value of the Host headers.
	TLSServerName  string          // The TLS server name to request a certificate for.
	RPCDestination string          // The hostname and port to send rpc federation requests to.
}

// ResolveServer implements the server name resolution algorithm described at
// https://matrix.org/docs/spec/server_server/r0.1.1.html#resolving-server-names
// Returns a slice of ResolutionResult that can be used to send a federation
// request to the server using a given server name.
// Returns an error if the server name isn't valid.
func ResolveServer(ctx context.Context, rpcServerName *spec.ServerName, serverName spec.ServerName) (results []ResolutionResult, err error) {
	return resolveServer(ctx, serverName, rpcServerName, true)
}

// resolveServer does the same thing as ResolveServer, except it also requires
// the checkWellKnown parameter, which indicates whether a .well-known file
// should be looked up.
func resolveServer(ctx context.Context, serverName spec.ServerName, rpcServerName *spec.ServerName, checkWellKnown bool) (results []ResolutionResult, err error) {
	host, port, valid := spec.ParseAndValidateServerName(serverName)
	if !valid {
		err = fmt.Errorf("Invalid server name")
		return
	}

	var rpcPort = -1
	var rpcHost = ""
	if rpcServerName != nil {
		rpcHost, rpcPort, valid = spec.ParseAndValidateServerName(*rpcServerName)
		if !valid {
			err = fmt.Errorf("Invalid rpc server name")
			return
		}
	}

	// 1. If the hostname is an IP literal
	// Check if we're dealing with an IPv6 literal with square brackets. If so,
	// remove the brackets.
	if host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	if net.ParseIP(host) != nil {
		var destination string

		if port == -1 {
			destination = net.JoinHostPort(host, strconv.Itoa(8448))
		} else {
			destination = string(serverName)
		}

		results = []ResolutionResult{
			{
				Destination:   destination,
				Host:          serverName,
				TLSServerName: host,
			},
		}

		if rpcServerName == nil {
			return
		}
	}

	// 2. Repeat step 1 for rpc
	if rpcServerName != nil {
		if rpcHost[0] == '[' && rpcHost[len(rpcHost)-1] == ']' {
			rpcHost = rpcHost[1 : len(rpcHost)-1]
		}
		if net.ParseIP(rpcHost) != nil {
			var rpcDestination string

			if rpcPort == -1 {
				rpcDestination = net.JoinHostPort(rpcHost, strconv.Itoa(8449))
			} else {
				rpcDestination = string(*rpcServerName)
			}

			results[0].RPCDestination = rpcDestination

			return
		}
	}

	// 3. If the hostname is not an IP literal, and the server name includes an
	// explicit port
	if port != -1 {
		results = []ResolutionResult{
			{
				Destination:   string(serverName),
				Host:          serverName,
				TLSServerName: host,
			},
		}

		if rpcServerName == nil {
			return
		}
	}

	// 4. Repeat step 4 for the rpc port
	if rpcPort != -1 {
		results[0].RPCDestination = string(*rpcServerName)

		return
	}

	if checkWellKnown {
		// 5. If the hostname is not an IP literal
		var result *WellKnownResult
		result, err = LookupWellKnown(ctx, serverName)
		if err == nil {
			// We don't want to check .well-known on the result
			return resolveServer(ctx, result.NewAddress, result.RpcAddress, false)
		}
	}

	return handleNoWellKnown(ctx, serverName, rpcServerName), nil
}

// handleNoWellKnown implements steps 4 and 5 of the resolution algorithm (as
// well as 3.3 and 3.4)
func handleNoWellKnown(ctx context.Context, serverName spec.ServerName, rpcServerName *spec.ServerName) (results []ResolutionResult) {
	// 4. If the /.well-known request resulted in an error response
	// 4a. Srv name support intentionally not added for rpc
	records, err := lookupSRV(ctx, serverName)
	if err == nil && len(records) > 0 {
		for _, rec := range records {
			// If the domain is a FQDN, remove the trailing dot at the end. This
			// isn't critical to send the request, as Go's HTTP client and most
			// servers understand FQDNs quite well, but it makes automated
			// testing easier.
			target := rec.Target
			if target[len(target)-1] == '.' {
				target = target[:len(target)-1]
			}

			var RPCDestination string
			if rpcServerName == nil {
				RPCDestination = fmt.Sprintf("%s:%d", target, 8449)
			} else {
				RPCDestination = fmt.Sprintf("%s:%d", *rpcServerName, 8449)
			}

			results = append(results, ResolutionResult{
				Destination:    fmt.Sprintf("%s:%d", target, rec.Port),
				Host:           serverName,
				TLSServerName:  string(serverName),
				RPCDestination: RPCDestination,
			})
		}

		return
	}

	// 5. If the /.well-known request returned an error response, and the SRV
	// record was not found
	var RPCDestination string
	if rpcServerName == nil {
		RPCDestination = fmt.Sprintf("%s:%d", serverName, 8449)
	} else {
		RPCDestination = fmt.Sprintf("%s:%d", *rpcServerName, 8449)
	}

	results = []ResolutionResult{
		{
			Destination:    fmt.Sprintf("%s:%d", serverName, 8448),
			Host:           serverName,
			TLSServerName:  string(serverName),
			RPCDestination: RPCDestination,
		},
	}

	return
}

func lookupSRV(ctx context.Context, serverName spec.ServerName) ([]*net.SRV, error) {
	// Check matrix-fed service first, as of Matrix 1.8
	_, records, err := net.DefaultResolver.LookupSRV(ctx, "matrix-fed", "tcp", string(serverName))
	if err != nil {
		if dnserr, ok := err.(*net.DNSError); ok {
			if !dnserr.IsNotFound {
				// not found errors are expected, but everything else is very much not
				return records, err
			}
		} else {
			return records, err
		}
	} else {
		return records, nil // we got a hit on the matrix-fed service, so use that
	}

	// we didn't get a hit on matrix-fed, so try deprecated matrix service
	_, records, err = net.DefaultResolver.LookupSRV(ctx, "matrix", "tcp", string(serverName))
	return records, err // we don't need to process this here
}
