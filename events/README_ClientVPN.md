# Sample Function

The following is a sample Lambda function that receives a Client VPN connection handler request as an input and then validates the IP address input and checks whether the connection source IP is on the allowed list defined as a map inside the function. If the source IP matches an allowed IP address it allows the access, otherwise an error message is presented to the user. Debug logs are generated to CloudWatch Logs. (Note that by default anything written to Console will be logged as CloudWatch Logs events.)

```go
import (
	"fmt"
	"log"
	"net"

	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	AllowedIPs = map[string]bool{
		"10.11.12.13": true,
	}
)

func handler(request events.ClientVPNConnectionHandlerRequest) (events.ClientVPNConnectionHandlerResponse, error) {
	requestJson, _ := json.MarshalIndent(request, "", "  ")
	log.Printf("REQUEST: %s", requestJson)

	sourceIP := request.PublicIP
	if net.ParseIP(sourceIP) == nil {
		return events.ClientVPNConnectionHandlerResponse{}, fmt.Errorf("Invalid parameter PublicIP passed in request: '%s'", sourceIP)
	}

	log.Printf("SOURCE IP: %s", sourceIP)

	if allowed, ok := AllowedIPs[sourceIP]; ok && allowed {
		log.Printf("Allowing access from: %s", sourceIP)
		return events.ClientVPNConnectionHandlerResponse{
			Allow: true,
			ErrorMsgOnFailedPostureCompliance: "",
			PostureComplianceStatuses: []string{},
			SchemaVersion: "v1",
		}, nil
	}

	log.Printf("Blocking access from: %s", sourceIP)
	return events.ClientVPNConnectionHandlerResponse{
		Allow: false,
		ErrorMsgOnFailedPostureCompliance: "You're accessing from a blocked IP range.",
		PostureComplianceStatuses: []string{"BlockedSourceIP"},
		SchemaVersion: "v1",
	}, nil
}

func main() {
	lambda.Start(handler)
}
```