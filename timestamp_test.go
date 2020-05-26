package grpcj

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
)

type SampleTimestampContainingStruct struct {
	T1 *timestamp.Timestamp
	T2 timestamp.Timestamp
}

func (s *SampleTimestampContainingStruct) Reset() { *s = SampleTimestampContainingStruct{} }
func (s *SampleTimestampContainingStruct) String() string {
	return fmt.Sprintf("%s,%s", s.T1.String(), s.T2.String())
}
func (*SampleTimestampContainingStruct) ProtoMessage() {}
func (*SampleTimestampContainingStruct) Descriptor() ([]byte, []int) {
	return []byte{1, 2, 3, 4, 5, 6, 7}, []int{0}
}

func TestTimeStampMarshaling(t *testing.T) {
	specDatetime := time.Date(1986, time.March, 10, 5, 5, 5, 5, time.UTC)
	seconds := specDatetime.Unix()
	nanos := specDatetime.Nanosecond()

	instance := SampleTimestampContainingStruct{
		T1: &timestamp.Timestamp{Seconds: seconds, Nanos: int32(nanos)},
		T2: timestamp.Timestamp{Seconds: seconds, Nanos: int32(nanos)},
	}
	expect_string := "{\"T1\":\"1986-03-10T05:05:05.000000005Z\",\"T2\":\"1986-03-10T05:05:05.000000005Z\"}"

	if res, err := DefaultMarshaler.MarshalToString(&instance); err != nil {
		t.Errorf("Error to Marshal the Structure, Error:%s", err)
	} else {
		if res != expect_string {
			t.Errorf("Expect: %s, Got: %s", expect_string, res)
		}
	}

}
