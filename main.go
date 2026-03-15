package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// Config holds all the values needed to provision the instance
type Config struct {
	CompartmentID        string
	SubnetID             string
	ImageID              string
	SSHPublicKey         string
	InstanceName         string
	OCPUs                float32
	MemoryGB             float32
	RetryIntervalSeconds int
}

func setupLogger() *os.File {
	os.MkdirAll("logs", 0755)
	logFileName := fmt.Sprintf("logs/provisioner-%s.log", time.Now().Format("2006-01-02"))
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("❌ Failed to open log file: %v", err)
	}
	// Write to both terminal AND log file simultaneously
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)
	return logFile
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
	// Setup logging to file + terminal
	logFile := setupLogger()
	defer logFile.Close()

	log.Println("🚀 OCI ARM A1 Instance Provisioner (Go)")
	log.Println("========================================")

	cfg := loadConfig()
	configProvider := common.DefaultConfigProvider()

	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		log.Fatalf("❌ Failed to create OCI compute client: %v", err)
	}

	ctx := context.Background()
	ads := getAvailabilityDomains(ctx, nil, cfg.CompartmentID)

	log.Printf("📋 Config:")
	log.Printf("   Instance Name : %s", cfg.InstanceName)
	log.Printf("   Shape         : VM.Standard.A1.Flex")
	log.Printf("   OCPUs         : %.0f", cfg.OCPUs)
	log.Printf("   Memory (GB)   : %.0f", cfg.MemoryGB)
	log.Printf("   Retry Interval: %d seconds", cfg.RetryIntervalSeconds)
	log.Printf("   ADs to try    : %v", ads)
	log.Println("========================================")

	attempt := 1
	for {
		log.Printf("🔄 Attempt #%d", attempt)

		for _, ad := range ads {
			log.Printf("   Trying AD: %s ...", ad)
			instance, err := tryCreateInstance(ctx, computeClient, cfg, ad)
			if err != nil {
				log.Printf("   ❌ Failed on %s: %v", ad, err)
				continue
			}

			// SUCCESS!
			log.Println("========================================")
			log.Printf("✅ SUCCESS! Instance created!")
			log.Printf("   Instance ID : %s", *instance.Id)
			log.Printf("   Name        : %s", *instance.DisplayName)
			log.Printf("   Shape       : %s", *instance.Shape)
			log.Printf("   AD          : %s", *instance.AvailabilityDomain)
			log.Printf("   State       : %s", instance.LifecycleState)
			log.Println("🎉 Check OCI Console for the public IP!")
			log.Println("========================================")
			os.Exit(0)
		}

		log.Printf("   ⏳ All ADs at capacity. Waiting %d seconds before next attempt...", cfg.RetryIntervalSeconds)
		time.Sleep(time.Duration(cfg.RetryIntervalSeconds) * time.Second)
		attempt++
	}
}
