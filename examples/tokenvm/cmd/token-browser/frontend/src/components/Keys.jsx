import {useEffect, useState} from "react";
import {GetKeys} from "../../wailsjs/go/main/App";
import MasterDetail from "./MasterDetail";
import {message} from "antd";

const Keys = () => {
    const [keys, setKeys] = useState([]);
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
        const getKeys = async () => {
            GetKeys()
                .then((keys) => {
                    setKeys(keys);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };
        getKeys();
    }, []);

    const title = "Keys";
    const getItemDescription = (key) => key;
    const detailLayout = (key) => (<KeyDetails key={key}/>);

    return (<>
            {contextHolder}
            <MasterDetail
                title={title}
                items={keys}
                getItemDescription={getItemDescription}
                detailLayout={detailLayout}
            />
        </>);
};

export default Keys;
