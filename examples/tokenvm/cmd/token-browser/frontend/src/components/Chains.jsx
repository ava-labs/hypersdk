import {useEffect, useState} from "react";
import {GetPublicRepositories} from "../../../wailsjs/go/main/App";
import MasterDetail from "../MasterDetail";
import {message} from "antd";

const Chains = () => {
    const [repositories, setRepositories] = useState([]);
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
        const getRepositories = async () => {
            GetPublicRepositories()
                .then((repositories) => {
                    setRepositories(repositories);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };
        getRepositories();
    }, []);

    const title = "Chains";
    const getItemDescription = (repository) => repository.name;
    const detailLayout = (repository) => (<ChainDetails repository={repository}/>);

    return (<>
            {contextHolder}
            <MasterDetail
                title={title}
                items={repositories}
                getItemDescription={getItemDescription}
                detailLayout={detailLayout}
            />
        </>);
};

export default Chains;
