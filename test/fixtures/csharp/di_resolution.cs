using System;

namespace Payment.Controllers
{
    public interface IPaymentService
    {
        void ProcessPayment();
    }

    public class PaymentService : IPaymentService
    {
        public void ProcessPayment() { }
    }

    public class PaymentController
    {
        private readonly IPaymentService _paymentService;

        public PaymentController(IPaymentService paymentService)
        {
            _paymentService = paymentService;
        }

        public void Post()
        {
            _paymentService.ProcessPayment();
        }
    }
}
