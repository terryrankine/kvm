package kvm

import (
	"fmt"

	"kvm/internal/network"
	"kvm/internal/udhcpc"

	"github.com/guregu/null/v6"
)

const (
	NetIfName = "eth0"
)

var (
	networkState *network.NetworkInterfaceState
)

func networkStateChanged(isOnline bool) {
	// do not block the main thread
	go waitCtrlAndRequestDisplayUpdate(true)

	if timeSync != nil {
		if networkState != nil {
			timeSync.SetDhcpNtpAddresses(networkState.NtpAddressesString())
		}

		if err := timeSync.Sync(); err != nil {
			networkLogger.Error().Err(err).Msg("failed to sync time after network state change")
		}
	}

	// always restart mDNS when the network state changes
	if mDNS != nil {
		_ = mDNS.SetListenOptions(config.NetworkConfig.GetMDNSMode())
		_ = mDNS.SetLocalNames([]string{
			networkState.GetHostname(),
			networkState.GetFQDN(),
		}, true)
	}

	// if the network is now online, trigger an NTP sync if still needed
	if isOnline && timeSync != nil && (isTimeSyncNeeded() || !timeSync.IsSyncSuccess()) {
		if err := timeSync.Sync(); err != nil {
			logger.Warn().Str("error", err.Error()).Msg("unable to sync time on network state change")
		}
	}
}

func initNetwork() error {
	ensureConfigLoaded()

	state, err := network.NewNetworkInterfaceState(&network.NetworkInterfaceOptions{
		DefaultHostname: GetDefaultHostname(),
		InterfaceName:   NetIfName,
		NetworkConfig:   config.NetworkConfig,
		Logger:          networkLogger,
		OnStateChange: func(state *network.NetworkInterfaceState) {
			networkStateChanged(state.IsOnline())
		},
		OnInitialCheck: func(state *network.NetworkInterfaceState) {
			networkStateChanged(state.IsOnline())
		},
		OnDhcpLeaseChange: func(lease *udhcpc.Lease, state *network.NetworkInterfaceState) {
			networkStateChanged(state.IsOnline())

			sess := getSession()
			if sess == nil {
				return
			}

			writeJSONRPCEvent("networkState", networkState.RpcGetNetworkState(), sess)
		},
		OnConfigChange: func(networkConfig *network.NetworkConfig) {
			config.NetworkConfig = networkConfig
			config.AppliedNetworkConfig = networkConfig
			networkStateChanged(false)

			if mDNS != nil {
				_ = mDNS.SetListenOptions(networkConfig.GetMDNSMode())
				_ = mDNS.SetLocalNames([]string{
					networkState.GetHostname(),
					networkState.GetFQDN(),
				}, true)
			}
		},
	})

	if state == nil {
		if err == nil {
			return fmt.Errorf("failed to create NetworkInterfaceState")
		}
		return err
	}

	if err := state.Run(); err != nil {
		return err
	}

	networkState = state

	if config != nil && config.NetworkConfig != nil {
		if config.NetworkConfig.PendingReboot.Valid && config.NetworkConfig.PendingReboot.Bool {
			if config.AppliedNetworkConfig != nil && network.IsSame(config.AppliedNetworkConfig, *config.NetworkConfig) {
				config.NetworkConfig.PendingReboot = null.BoolFrom(false)
				_ = SaveConfig()
			}
		}
	}

	if config != nil && config.NetworkConfig != nil {
		if config.NetworkConfig.PendingReboot.Valid && config.NetworkConfig.PendingReboot.Bool {
			config.NetworkConfig.PendingReboot = null.BoolFrom(false)
			_ = SaveConfig()
		}
	}

	return nil
}

func rpcGetNetworkState() network.RpcNetworkState {
	return networkState.RpcGetNetworkState()
}

func rpcGetNetworkSettings() network.RpcNetworkSettings {
	rpcSettings := networkState.RpcGetNetworkSettings()
	hostname := GetHostname()
	rpcSettings.Hostname = null.NewString(hostname, hostname != "")
	return rpcSettings
}

func rpcSetNetworkSettings(settings network.RpcNetworkSettings) (*network.RpcNetworkSettings, error) {
	current := networkState.RpcGetNetworkSettings()
	changedCore := !network.IsSame(current.NetworkConfig, settings.NetworkConfig)
	if changedCore {
		settings.NetworkConfig.PendingReboot = null.BoolFrom(true)
	}

	s := networkState.RpcSetNetworkSettings(settings)
	if s != nil {
		return nil, s
	}
	applied := networkState.RpcGetNetworkSettings()
	config.NetworkConfig = &applied.NetworkConfig
	// If we just reverted to the same core config as applied, clear pending_reboot
	if config.AppliedNetworkConfig != nil {
		// create copies ignoring PendingReboot
		a := *config.AppliedNetworkConfig
		b := applied.NetworkConfig
		a.PendingReboot = null.Bool{}
		b.PendingReboot = null.Bool{}
		if network.IsSame(a, b) {
			config.NetworkConfig.PendingReboot = null.BoolFrom(false)
		}
	}
	if err := SaveConfig(); err != nil {
		return nil, err
	}

	return &network.RpcNetworkSettings{NetworkConfig: *config.NetworkConfig}, nil
}

func rpcRenewDHCPLease() error {
	return networkState.RpcRenewDHCPLease()
}

func rpcRequestDHCPAddress(ip string) error {
	return networkState.RpcRequestDHCPAddress(ip)
}
