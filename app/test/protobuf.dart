import '../lib/src/protos/authstore.pb.dart';

void newAuthKey(){
  var ak = AuthKey();
  ak.writeToJson();
}