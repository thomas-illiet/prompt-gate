package config

import (
	"fmt"
	"net"
	"strconv"
)

func validateHealthAddress(field, address string) error {
	_, port, err := net.SplitHostPort(address)
	number, parseErr := strconv.Atoi(port)
	if err != nil || parseErr != nil || number < 1 || number > 65535 {
		return fmt.Errorf("%s must be a host:port address with a port between 1 and 65535", field)
	}
	return nil
}
