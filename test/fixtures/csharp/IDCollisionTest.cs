namespace MyApp.Core;

public class User {
    public string Name;
    public User() { }
    public User(string name) { Name = name; }
    public void Process() { }
    public void Process(int id) { }
    public void Process(string data, int options) { }
    
    // Field vs Method collision
    public int count;
    public int count_method() { return 0; }
}

