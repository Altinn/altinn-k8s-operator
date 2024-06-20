package operatorcontext

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDiscoversOk(t *testing.T) {
	RegisterTestingT(t)

	operatorContext, err := Discover(context.Background())
	Expect(err).NotTo(HaveOccurred())
	Expect(operatorContext).NotTo(BeNil())
}

func TestCancellationBefore(t *testing.T) {
	RegisterTestingT(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	operatorContext, err := Discover(ctx)
	Expect(operatorContext).To(BeNil())
	Expect(ctx.Err()).To(MatchError("context canceled"))
	Expect(err).To(MatchError("context canceled"))

}

func TestCancellationAfter(t *testing.T) {
	RegisterTestingT(t)

	ctx, cancel := context.WithCancel(context.Background())
	operatorContext, err := Discover(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(operatorContext).NotTo(BeNil())
	Expect(ctx.Err()).To(Succeed())

	cancel()
	Expect(ctx.Err()).To(MatchError("context canceled"))
}
