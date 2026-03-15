package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// Config holds all the values needed to provision the instance
type Config struct {
	// OCI credentials (from ~/.oci/config or env vars)
	CompartmentID string
	SubnetID      string
	ImageID       string
	SSHPublicKey  string

	// Instance settings
	InstanceName string
	OCPUs        float32
	MemoryGB     float32

	// Retry settings
	RetryIntervalSeconds int
}

func loadConfig() Config {
	return Config{
		CompartmentID:        mustEnv("OCI_COMPARTMENT_ID"),
		SubnetID:             mustEnv("OCI_SUBNET_ID"),
		ImageID:              mustEnv("OCI_IMAGE_ID"),
		SSHPublicKey:         mustEnv("OCI_SSH_PUBLIC_KEY"),
		InstanceName:         getEnvOrDefault("OCI_INSTANCE_NAME", "arm-instance"),
		OCPUs:                float32(getEnvFloat("OCI_OCPUS", 2)),
		MemoryGB:             float32(getEnvFloat("OCI_MEMORY_GB", 12)),
		RetryIntervalSeconds: int(getEnvFloat("OCI_RETRY_INTERVAL_SECONDS", 300)),
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ Required environment variable %s is not set", key)
	}
	return val
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

func tryCreateInstance(ctx context.Context, client core.ComputeClient, cfg Config, ad string) (*core.Instance, error) {
	ocpus := cfg.OCPUs
	memGB := cfg.MemoryGB
	shape := "VM.Standard.A1.Flex"

	req := core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			CompartmentId: common.String(cfg.CompartmentID),
			DisplayName:   common.String(cfg.InstanceName),
			Shape:         common.String(shape),
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus:       &ocpus,
				MemoryInGBs: &memGB,
			},
			AvailabilityDomain: common.String(ad),
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       common.String(cfg.SubnetID),
				AssignPublicIp: common.Bool(true),
			},
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId: common.String(cfg.ImageID),
			},
			Metadata: map[string]string{
				"ssh_authorized_keys": cfg.SSHPublicKey,
			},
		},
	}

	resp, err := client.LaunchInstance(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.Instance, nil
}

func getAvailabilityDomains(ctx context.Context, identityClient interface{}, compartmentID string) []string {
	// Return all 3 ADs to try - OCI always has AD-1, AD-2, AD-3
	// You can hardcode your region's ADs here if needed
	// e.g. for us-ashburn-1: IQvU:US-ASHBURN-AD-1, etc.
	// We'll read from env or use a generic approach
	adsEnv := os.Getenv("OCI_AVAILABILITY_DOMAINS")
	if adsEnv != "" {
		// If user provides comma-separated ADs
		ads := []string{}
		start := 0
		for i := 0; i <= len(adsEnv); i++ {
			if i == len(adsEnv) || adsEnv[i] == ',' {
				ad := adsEnv[start:i]
				if ad != "" {
					ads = append(ads, ad)
				}
				start = i + 1
			}
		}
		return ads
	}
	// Default: return placeholder - user must set OCI_AVAILABILITY_DOMAINS
	log.Fatal("❌ Please set OCI_AVAILABILITY_DOMAINS as comma-separated list, e.g.:\n  export OCI_AVAILABILITY_DOMAINS=\"IQvU:US-ASHBURN-AD-1,IQvU:US-ASHBURN-AD-2,IQvU:US-ASHBURN-AD-3\"")
	return nil
}

func main() {
	fmt.Println("🚀 OCI ARM A1 Instance Provisioner (Go)")
	fmt.Println("========================================")

	cfg := loadConfig()

	// Load OCI config from ~/.oci/config (default profile)
	configProvider := common.DefaultConfigProvider()

	// Create compute client
	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		log.Fatalf("❌ Failed to create OCI compute client: %v", err)
	}

	ctx := context.Background()

	// Get availability domains to try
	ads := getAvailabilityDomains(ctx, nil, cfg.CompartmentID)

	fmt.Printf("📋 Config:\n")
	fmt.Printf("   Instance Name : %s\n", cfg.InstanceName)
	fmt.Printf("   Shape         : VM.Standard.A1.Flex\n")
	fmt.Printf("   OCPUs         : %.0f\n", cfg.OCPUs)
	fmt.Printf("   Memory (GB)   : %.0f\n", cfg.MemoryGB)
	fmt.Printf("   Retry Interval: %d seconds\n", cfg.RetryIntervalSeconds)
	fmt.Printf("   ADs to try    : %v\n\n", ads)

	attempt := 1
	for {
		fmt.Printf("🔄 Attempt #%d — %s\n", attempt, time.Now().Format("2006-01-02 15:04:05"))

		for _, ad := range ads {
			fmt.Printf("   Trying availability domain: %s ... ", ad)
			instance, err := tryCreateInstance(ctx, computeClient, cfg, ad)
			if err != nil {
				fmt.Printf("❌ Failed: %v\n", err)
				continue
			}

			// SUCCESS!
			fmt.Printf("\n✅ SUCCESS! Instance created!\n")
			fmt.Printf("   Instance ID  : %s\n", *instance.Id)
			fmt.Printf("   Display Name : %s\n", *instance.DisplayName)
			fmt.Printf("   Shape        : %s\n", *instance.Shape)
			fmt.Printf("   AD           : %s\n", *instance.AvailabilityDomain)
			fmt.Printf("   State        : %s\n", instance.LifecycleState)
			fmt.Println("\n🎉 Your ARM instance is being provisioned. Check the OCI console for the public IP.")
			os.Exit(0)
		}

		fmt.Printf("   All ADs are at capacity. Waiting %d seconds before retry...\n\n", cfg.RetryIntervalSeconds)
		time.Sleep(time.Duration(cfg.RetryIntervalSeconds) * time.Second)
		attempt++
	}
}
