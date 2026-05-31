//go:build mimir_eval_fixture

package backend

type CheckoutService struct{}

func (s *CheckoutService) PlaceOrder(amount int) error {
	return Charge(amount)
}
