using Microsoft.Extensions.Logging;
using Trucks.Common;

namespace Trucks.Processor;

public class PaymentProcessor
{
    private readonly ILogger<PaymentProcessor> _logger;
    private readonly IPaymentRepository _repository;

    public PaymentProcessor(ILogger<PaymentProcessor> logger, IPaymentRepository repository)
    {
        _logger = logger;
        _repository = repository;
    }

    public void Process()
    {
        _logger.LogInformation("Processing payment");
        _repository.SavePayment();
    }
}

public interface IPaymentRepository
{
    void SavePayment();
}
