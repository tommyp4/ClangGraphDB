import { UserService } from './user.service';
import { Logger } from './logger';
import { Repository } from './repository';

export class DISample {
  // Field Injection
  private logger: Logger;
  
  // Generic Field
  private repos: Repository<User>;

  // Constructor Injection (Angular/NestJS style)
  constructor(
    private userService: UserService,
    logger: Logger // Parameter without access modifier (still a dependency)
  ) {
    this.logger = logger;
  }

  doWork() {
    this.userService.process();
  }
}
