import {useEffect, useState} from "react";
import {GetLatestBlocks} from "../../wailsjs/go/main/App";
import {List, message} from "antd";

const Explorer = () => {
    const [blocks, setBlocks] = useState([]);
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
        const getLatestBlocks = async () => {
            GetLatestBlocks()
                .then((blocks) => {
                    setBlocks(blocks);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const interval = setInterval(() => {
          getLatestBlocks();
        }, 500);

        return () => clearInterval(interval);
    }, []);

    return (<>
            {contextHolder}
            <List
              bordered
              dataSource={blocks}
              renderItem={(item) => (
                <List.Item>
                  {item.Str}
                </List.Item>
              )}
            />
        </>);
};

export default Explorer;
