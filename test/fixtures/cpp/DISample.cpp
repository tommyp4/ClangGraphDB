#include <vector>
#include "UserService.h"
#include "UserRepository.h"

class DISample {
private:
    UserService* userService;
    std::vector<UserRepository> repositories;

public:
    DISample(UserService* service, std::vector<UserRepository> repos) 
        : userService(service), repositories(repos) {
    }

    void doWork() {
        userService->process();
    }
};
