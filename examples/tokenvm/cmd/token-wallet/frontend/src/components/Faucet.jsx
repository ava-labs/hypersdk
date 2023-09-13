import {useEffect, useState} from "react";
import { Divider, List, Card, Typography, Form, Input, InputNumber, Button, Select, message } from "antd";
import { CheckCircleTwoTone, CloseCircleTwoTone } from '@ant-design/icons';
import { StartFaucetSearch, GetFaucetSolutions } from "../../wailsjs/go/main/App";
const { Text, Title, Link } = Typography;

const Faucet = () => {
    const [messageApi, contextHolder] = message.useMessage();
    const [loaded, setLoaded] = useState(false);
    const [search, setSearch] = useState(null);
    const [solutions, setSolutions] = useState([]);

    const startSearch = async () => {
      const newSearch = await StartFaucetSearch();
      setSearch(newSearch);
    };

    useEffect(() => {
      const getFaucetSolutions = async () => {
          const faucetSolutions = await GetFaucetSolutions();
          if (faucetSolutions.Alert) {
            const lastSolution = faucetSolutions.PastSearches[0];
            if (lastSolution.Err.length == 0) {
              messageApi.open({
                type: "success", content: `Search Successful (attempts: ${lastSolution.Attempts}, amount: ${lastSolution.Amount})`,
              });
            } else {
              messageApi.open({
                type: "error", content: `Search Unsucessful: ${lastSolution.Err}`,
              });
            }
          }
          setSearch(faucetSolutions.CurrentSearch);
          setSolutions(faucetSolutions.PastSearches);
          setLoaded(true);
      };

      getFaucetSolutions();
      const interval = setInterval(() => {
        getFaucetSolutions();
      }, 500);

      return () => clearInterval(interval);
    }, []);

    return (<>
            {contextHolder}
            <Card bordered title={"Mine for Tokens"} style={{ margin: "auto" }}>
              {loaded && search === null &&
                <div>
                  <Text italic>To get TKN, you must complete a PoW.</Text>
                  <Button type="primary" style={{ width:"100%" }} onClick={startSearch}>Start</Button>
                </div>
              }
              {loaded && search !== null &&
                <div>
                  <Text italic>search running (you can leave this page and come back)</Text>
                  <br />
                  <Text strong>Faucet Address:</Text>{search.FaucetAddress}
                  <br />
                  <Text strong>Salt:</Text>{search.Salt}
                  <br />
                  <Text strong>Difficulty:</Text>{search.Difficulty}
                </div>
              }
            </Card>

      <Divider orientation="center">
        Previous Solutions
      </Divider>
      <List
        bordered
        dataSource={solutions}
        renderItem={(item) => (
          <List.Item>
            {item.Err.length > 0 &&
              <div>
                <div>
                  <Text strong>{item.Solution} </Text>
                  <CloseCircleTwoTone twoToneColor="red" />
                </div>
                <Text strong>Salt:</Text> {item.Salt}
                <br />
                <Text strong>Difficulty:</Text> {item.Difficulty}
                <br />
                <Text strong>Attempts:</Text> {item.Attempts}
                <br />
                <Text strong>Elapsed:</Text> {item.Elapsed}
                <br />
                <Text strong>Error:</Text> {item.Err}
              </div>
            }
            {item.Err.length == 0 &&
              <div>
                <div>
                  <Text strong>{item.Solution} </Text>
                  <CheckCircleTwoTone twoToneColor="green" />
                </div>
                <Text strong>Salt:</Text> {item.Salt}
                <br />
                <Text strong>Difficulty:</Text> {item.Difficulty}
                <br />
                <Text strong>Attempts:</Text> {item.Attempts}
                <br />
                <Text strong>Elapsed:</Text> {item.Elapsed}
                <br />
                <Text strong>Amount:</Text> {item.Amount}
                <br />
                <Text strong>TxID:</Text> {item.TxID}
              </div>
            }
          </List.Item>
        )}
      />
    </>);
}
export default Faucet;

{/* Previous faucet attempts as list, current faucet attempt, start new faucet attempt */}
