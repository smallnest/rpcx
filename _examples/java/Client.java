import java.io.*;
import java.net.Socket;
import java.net.UnknownHostException;

public class Client {

    public static void main(String[] args) {
        String hostName = "127.0.0.1"; //args[0];
        int portNumber = 8972; //Integer.parseInt(args[1]);

        try {
            Socket socket = new Socket(hostName, portNumber);


            PrintWriter out = new PrintWriter(socket.getOutputStream(), true);

            String r = "{\"jsonrpc\": \"2.0\", \"method\": \"Arith.Mul\", \"params\": {\"A\":7,\"B\":8}, \"id\": 1}";

            out.write(r);
            out.flush();

            System.out.println(r);
            BufferedReader in = new BufferedReader(new InputStreamReader(socket.getInputStream()));
            String s;
            while ((s = in.readLine()) != null) {
                System.out.printf(s);
            }

        } catch (UnknownHostException e) {
            e.printStackTrace();
        } catch (IOException e) {
            e.printStackTrace();
        } catch (Exception ex) {
            ex.printStackTrace();
        }
    }
}