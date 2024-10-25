package scaling

import (
	"github.com/stretchr/testify/mock"
)

type updaterMock struct {
	mock.Mock
}

func (u *updaterMock) Update(opts *CPUScalingOpts) {
	u.Called(opts)
}
