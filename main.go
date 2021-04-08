
package main

import (

	"fmt"	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"

	"github.com/olekukonko/tablewriter"

	"os"

	"strings"

)

// TraceableRegions is a list of AWS regions we want to crawl

//var TraceableRegions = [...]string{"us-east-1", "us-east-2", "us-west-1", "us-west-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-southeast-1", "ap-southeast-2", "ap-south-1", "sa-east-1"}

var TraceableRegions = [...]string{"eu-west-1"}

// SingleResource defines how we want to describe each AWS resource

type SingleResource struct {

	Region  *string

	Service *string

	Product *string

	Details *string

	ID      *string

	ARN     *string

}

// PrettyPrintResources makes use of a nice golang library to show

// tables on stdout. Check it out: github.com/olekukonko/tablewriter

func PrettyPrintResources(resources []*SingleResource) {

	var data [][]string

	for _, r := range resources {

		row := []string{

			DerefNilPointerStrings(r.Region),

			DerefNilPointerStrings(r.Service),

			DerefNilPointerStrings(r.Product),

			DerefNilPointerStrings(r.ID),

		}

		data = append(data, row)

	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Region", "Service", "Product", "ID"})

	table.SetBorder(true) // Set Border to false

	table.AppendBulk(data)

	table.Render()

}

// GetServiceFromArn removes the arn:aws: component string of

// the name and returns the first keyword that appears, svc

func ServiceNameFromARN(arn *string) *string {

	shortArn := strings.Replace(*arn, "arn:aws:", "", -1)

	sliced := strings.Split(shortArn, ":")

	return &sliced[0]

}

// Short ARN removes the unnecessary info from the ARN we already

// know at this point like region, account id and the service name.

func ShortArn(arn *string) string {

	slicedArn := strings.Split(*arn, ":")

	shortArn := slicedArn[5:]  // the first 5 we already have

	return strings.Join(shortArn, "/")

}

// awsEC2 type is created for ARNs belonging to the EC2 service

type awsEC2 string

// awsECS type is created for ARNs belonging to the ECS service

type awsECS string

// awsGeneric is a is a generic AWS for services ARNs that don't have

// a dedicated type within our application.

type awsGeneric string

// Generic Resource Handler

func (aws *awsGeneric) ConverToResource(shortArn, svc, rgn *string) *SingleResource {

	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, ID: shortArn,}

}

// ConvertToRow converts EC2 shortened ARNs to to a SingleResource type

func (aws *awsEC2) ConvertToResource(shortArn, svc, rgn *string) *SingleResource {

	// ec2    			instance/i-23123jj1k1k23jh12

	// ec2    			security-group/sg-23bn1m231233123m1

	// ec2    			subnet/subnet-92i3i1i23i1ih1v23

	// ec2    			vpc/vpc-12i3o1ijkj12jh123

	s := strings.Split(*shortArn, "/")

	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, Product: &s[0], ID: &s[1],}

}

// ConvertToRow converts ECS shortened ARNs to to a SingleResource type

func (aws *awsECS) ConvertToResource(shortArn, svc, rgn *string) *SingleResource {

	// ecs    			cluster/some-ecs-cluster

	s := strings.Split(*shortArn, "/")

	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, Product: &s[0], ID: &s[1],}

}

// GetResourceRow shortens the ARN and assigns it to the right

// service type calling its "ConvertToRow" method. Since we have

// a default behaviour funneled towards our awsGeneric type, all

// services will be handled.

func ConvertArnToSingleResource(arn, svc, rgn *string) *SingleResource {

	shortArn := ShortArn(arn)

	switch *svc {

	case "ec2":

		res := awsEC2(*svc)

		return res.ConvertToResource(&shortArn, svc, rgn)

	case "ecs":

		res := awsECS(*svc)

		return res.ConvertToResource(&shortArn, svc, rgn)

	default:

		res := awsGeneric(*svc)

		return res.ConverToResource(&shortArn, svc, rgn)

	}

}

// DerefNilPointerStrings utility func to make sure we don't run into

// a "nil pointer dereference" issue during runtime.

func DerefNilPointerStrings(s *string) string {

	if s == nil {

		return ""

	}

	return *s

}

func main() {

	var resources []*SingleResource

	for _, region := range TraceableRegions {

		// We need to create a new CFG for each region. We

		// could actually update the region after the fact

		// but let's focus on the purpose, here :)

		cfg := aws.Config{Region: aws.String(region)}

		s := session.Must(session.NewSessionWithOptions(session.Options{

			SharedConfigState: session.SharedConfigEnable,

			Config:            cfg,

		}))

		// Creating the actual AWS client from the SDK

		r := resourcegroupstaggingapi.New(s)

		// The results will come paginated, so we create an empty

		// one outside the next for loop so we can keep updating

		// it and check if there are still more results to come or

		// not. We could isolate this function and call it recursively

		// if we wanted to tidy up our code.

		var paginationToken string = ""

		var in *resourcegroupstaggingapi.GetResourcesInput

		var out *resourcegroupstaggingapi.GetResourcesOutput

		var err error

		// Let's start an infinite for loop until there are no

		for {

			if len(paginationToken) == 0 {

				in = &resourcegroupstaggingapi.GetResourcesInput{

					ResourcesPerPage: aws.Int64(50),

				}

				out, err = r.GetResources(in)

				if err != nil {

					fmt.Println(err)

				}

			} else {

				in = &resourcegroupstaggingapi.GetResourcesInput{

					ResourcesPerPage: aws.Int64(50),

					PaginationToken:  &paginationToken,

				}

			}

			out, err = r.GetResources(in)

			if err != nil {

				fmt.Println(err)

			}

			for _, resource := range out.ResourceTagMappingList {

				svc := ServiceNameFromARN(resource.ResourceARN)

				rgn := region

				resources = append(resources, ConvertArnToSingleResource(resource.ResourceARN, svc, &rgn))

			}

			paginationToken = *out.PaginationToken

			if *out.PaginationToken == "" {

				break

			}

		}

	}

	// Finally print the results

	PrettyPrintResources(resources)

}
