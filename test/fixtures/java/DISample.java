package com.example.di;

import java.util.List;
import com.example.services.UserService;
import com.example.repositories.UserRepository;

public class DISample {
    private final UserService userService;
    private final List<UserRepository> repositories;

    public DISample(UserService userService, List<UserRepository> repositories) {
        this.userService = userService;
        this.repositories = repositories;
    }

    public void doWork() {
        userService.process();
    }
}
